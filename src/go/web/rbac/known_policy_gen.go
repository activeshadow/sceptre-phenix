//go:generate go run known_policy_gen.go
//go:build ignore

// This program generates known_policy.go. It can be invoked by running `go generate`
package main

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/exp/slices"
)

type Permission struct {
	Resource string
	Verb     string
}

var code = `// Code generated by go generate; DO NOT EDIT.
// This file was generated at build time.
// This contains all known role checks used in codebase.

package rbac

type Permission struct {
	Resource string
	Verb     string
}

var Permissions = []Permission{
{{- range .Permissions }}
	{"{{.Resource}}", "{{.Verb}}"},
{{- end }}
}
`
var packageTemplate = template.Must(template.New("").Parse(code))

func main() {
	var permissions []Permission

	out, err := exec.Command(
		"grep", "-hroPI",
		"(?<=role.Allowed.\")(.+?)\\)", "../../",
		"--exclude=*_test.go",
	).Output()

	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))

	for scanner.Scan() {
		var (
			parts = strings.Split(scanner.Text(), ",")
			pair  = Permission{strings.Trim(parts[0], "\"() "), strings.Trim(parts[1], "\"() ")}
		)

		if !slices.Contains(permissions, pair) {
			permissions = append(permissions, pair)
		}
	}

	// sort checks alphabetically
	sort.SliceStable(permissions, func(i, j int) bool {
		if permissions[i].Resource == permissions[j].Resource {
			return permissions[i].Verb < permissions[j].Verb
		}

		return permissions[i].Resource < permissions[j].Resource
	})

	f, err := os.Create("known_policy.go")
	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	packageTemplate.Execute(f, struct {
		Permissions []Permission
	}{
		Permissions: permissions,
	})
}
