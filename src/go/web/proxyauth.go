package web

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"phenix/util/plog"
	"phenix/web/middleware"
	"phenix/web/rbac"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"

	jwtutil "phenix/web/util/jwt"
)

type proxyAuth struct {
	jwtKey      string
	lifetime    time.Duration
	tokenHeader string
	userHeader  string
}

func newProxyAuth(o serverOptions) proxyAuth {
	return proxyAuth{
		jwtKey:      o.jwtKey,
		lifetime:    o.jwtLifetime,
		tokenHeader: o.authTokenHeader,
		userHeader:  o.proxiedUserHeader,
	}
}

func (this proxyAuth) Middleware() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/signup") || strings.HasSuffix(r.URL.Path, "/login") {
				h.ServeHTTP(w, r)

				return
			}

			authenticated, err := this.authenticateJWT(r)
			if err != nil {
				plog.Warn(plog.TypeSecurity, "rejecting unauthorized request", "path", r.URL.Path, "query", r.URL.RawQuery, "err", err)
				http.Error(w, "Forbidden", http.StatusUnauthorized)

				return
			}

			h.ServeHTTP(w, r.WithContext(authenticated))
		})
	}
}

func (this proxyAuth) Login(w http.ResponseWriter, r *http.Request) {
	plog.Debug(plog.TypeSystem, "HTTP handler called", "handler", "ProxyLogin")

	if r.Method != http.MethodGet {
		http.Error(w, "proxied auth enabled -- must login via GET request", http.StatusBadRequest)

		return
	}

	username := r.Header.Get(this.userHeader)
	if username == "" {
		plog.Error(plog.TypeSecurity, "proxy authentication failed")
		http.Error(w, "proxy authentication failed", http.StatusUnauthorized)

		return
	}

	u, err := rbac.GetUser(username)
	if err != nil {
		plog.Error(plog.TypeSecurity, "attempted proxy login with unknown username", "username", username)
		http.Error(w, username, http.StatusNotFound)

		return
	}

	this.writeLoginResponse(w, u)
}

func (this proxyAuth) Signup(w http.ResponseWriter, r *http.Request) {
	plog.Debug(plog.TypeSystem, "HTTP handler called", "handler", "ProxySignup")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		plog.Error(plog.TypeSystem, "reading request body", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	var req SignupRequest
	if err := json.Unmarshal(body, &req); err != nil {
		plog.Error(plog.TypeSystem, "unmarshaling request body", "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	username := r.Header.Get(this.userHeader)
	if username == "" {
		plog.Error(plog.TypeSecurity, "proxy signup without authenticated user")
		http.Error(w, "proxy authentication failed", http.StatusUnauthorized)

		return
	}

	if req.Username != username {
		http.Error(w, "proxy user mismatch", http.StatusUnauthorized)

		return
	}

	u := rbac.NewUser(req.Username, randomProxyPassword())
	if u == nil {
		http.Error(w, "error creating user", http.StatusInternalServerError)

		return
	}

	u.Spec.FirstName = req.FirstName
	u.Spec.LastName = req.LastName

	plog.Info(
		plog.TypeSecurity,
		"created proxy user",
		"user",
		u.Username(),
		"role",
		u.RoleName(),
		"first_name",
		u.FirstName(),
		"last_name",
		u.LastName(),
	)

	this.writeLoginResponse(w, u)
}

func (this proxyAuth) Logout(w http.ResponseWriter, r *http.Request) {
	Logout(w, r)
}

func (this proxyAuth) authenticateJWT(r *http.Request) (context.Context, error) {
	raw, err := tokenFromBearerHeader(r, this.tokenHeader)
	if err != nil {
		return nil, err
	}

	if raw == "" {
		raw = r.URL.Query().Get("token")
	}

	if raw == "" {
		return nil, fmt.Errorf("missing phenix auth token")
	}

	token, err := jwt.ParseWithClaims(raw, jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected JWT signing method: %s", token.Header["alg"])
		}

		return []byte(this.jwtKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("validating JWT: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid JWT")
	}

	username, err := jwtutil.UsernameFromClaims(claims)
	if err != nil {
		return nil, err
	}

	u, err := rbac.GetUser(username)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}

	if err := u.ValidateToken(raw); err != nil {
		return nil, fmt.Errorf("validating user token: %w", err)
	}

	role, err := u.Role()
	if err != nil {
		return nil, fmt.Errorf("getting user role: %w", err)
	}

	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.ContextKeyUser, u.Username())
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, role)
	ctx = context.WithValue(ctx, middleware.ContextKeyJWT, raw)

	return ctx, nil
}

func (this proxyAuth) writeLoginResponse(w http.ResponseWriter, u *rbac.User) {
	signed, err := this.issueToken(u)
	if err != nil {
		plog.Error(plog.TypeSecurity, "failed to issue proxy login token", "user", u.Username(), "err", err)
		http.Error(w, "failed to sign JWT", http.StatusInternalServerError)

		return
	}

	resp := LoginResponse{
		User:  userFromRBAC(*u),
		Token: signed,
	}

	body, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	plog.Info(plog.TypeSecurity, "user signed in via proxy", "user", u.Username())

	_, _ = w.Write(body)
}

func (this proxyAuth) issueToken(u *rbac.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": u.Username(),
		"exp": time.Now().Add(this.lifetime).Unix(),
	})

	signed, err := token.SignedString([]byte(this.jwtKey))
	if err != nil {
		return "", err
	}

	if err := u.AddToken(signed, time.Now().Format(time.RFC3339)); err != nil {
		return "", err
	}

	return signed, nil
}

func tokenFromBearerHeader(r *http.Request, header string) (string, error) {
	authHeader := r.Header.Get(header)
	if authHeader == "" {
		return "", nil
	}

	authHeaderParts := strings.Split(authHeader, " ")
	if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
		return "", fmt.Errorf("%s header format must be 'Bearer {token}'", header)
	}

	return authHeaderParts[1], nil
}

func randomProxyPassword() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format(time.RFC3339Nano)
	}

	return base64.RawStdEncoding.EncodeToString(b)
}
