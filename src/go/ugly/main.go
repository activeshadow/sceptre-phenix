package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"phenix/api/experiment"
	"phenix/api/vm"
	"phenix/store"
	"phenix/util/mm"
	"phenix/util/plog"
	"phenix/web/util"

	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/net/websocket"
)

var (
	basePath  string
	assetPath = "/opt/phenix/ugly"
)

func main() {
	plog.NewPhenixHandler()
	plog.SetLevelText("debug")

	switch len(os.Args) {
	case 2:
		assetPath = os.Args[1]
	case 3:
		assetPath = os.Args[1]
		basePath = os.Args[2]
	}

	tmplPath = assetPath + "/templates"

	store.Init(store.Endpoint("bolt:///etc/phenix/store.bdb"))

	router := mux.NewRouter().StrictSlash(true)

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, basePath+"/", http.StatusSeeOther)
	})

	router.PathPrefix("/images/").Handler(
		http.FileServer(http.Dir(assetPath + "/public")),
	)

	router.PathPrefix("/novnc/").Handler(
		http.FileServer(http.Dir(assetPath + "/public")),
	)

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var (
			params         = NewParams(r.FormValue("experiment"))
			vmFilter       = r.FormValue("vm-filter")
			experiments, _ = experiment.List()
		)

		params.Modal = r.FormValue("modal")

		plog.Debug("GET index", "exp", r.FormValue("experiment"), "vm-filter", vmFilter)

		for _, exp := range experiments {
			params.Experiments = append(params.Experiments, exp.Metadata.Name)

			if params.Experiment == exp.Metadata.Name {
				params.Topology = exp.Metadata.Annotations["topology"]
				params.Scenario = exp.Metadata.Annotations["scenario"]
				params.Running = exp.Running()

				var vms []mm.VM

				if exp.Running() {
					list, _ := vm.List(exp.Metadata.Name)

					for _, vm := range list {
						if vm.Running {
							vms = append(vms, vm)
						}
					}
				} else {
					vms, _ = vm.List(exp.Metadata.Name)
				}

				params.VMs = filterVMs(vms, vmFilter)
				params.VMFilter = vmFilter
			}
		}

		Index(w, params)
	})

	router.HandleFunc("/experiments/{name}", func(w http.ResponseWriter, r *http.Request) {
		var (
			name   = mux.Vars(r)["name"]
			action = r.FormValue("action")

			vms []string
		)

		if r.Form.Has("vms") {
			vms = r.Form["vms"]
		}

		exp, _ := experiment.Get(name)

		for _, name := range vms {
			vm := exp.Spec.Topology().FindNodeByName(name)

			switch action {
			case "boot":
				vm.General().SetDoNotBoot(false)
			case "dnb":
				vm.General().SetDoNotBoot(true)
			}
		}

		experiment.Save(experiment.SaveWithName(name), experiment.SaveWithSpec(exp.Spec))

		http.Redirect(w, r, fmt.Sprintf("%s/?experiment=%s&vm-filter=%s", basePath, name, r.FormValue("vm-filter")), http.StatusSeeOther)
	})

	// GET /experiments/{exp}/vms/{name}/vnc
	router.HandleFunc("/experiments/{exp}/vms/{name}/vnc", func(w http.ResponseWriter, r *http.Request) {
		plog.Debug("HTTP handler called", "handler", "GetVNC")

		var (
			vars = mux.Vars(r)
			exp  = vars["exp"]
			name = vars["name"]
		)

		vm, err := vm.Get(exp, name)
		if err != nil {
			http.Error(w, "VM not found", http.StatusNotFound)
			return
		}

		config := newVNCBannerConfig(exp, name)

		if banner, ok := vm.Annotations["vncBanner"]; ok {
			switch banner := banner.(type) {
			case string:
				config.finalize(banner)
			case map[string]interface{}:
				if err := mapstructure.Decode(banner, &config); err != nil {
					plog.Error("decoding vncBanner annotation for VM", "vm", name, "err", err)
				} else {
					config.finalize()
				}
			default:
				plog.Error("unexpected interface type for vncBanner annotation")
			}
		} else {
			config.finalize(fmt.Sprintf("EXP: %s - VM: %s", exp, name))
		}

		// set no-cache headers
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1.
		w.Header().Set("Pragma", "no-cache")                                   // HTTP 1.0.
		w.Header().Set("Expires", "0")                                         // Proxies.

		tmpl := template.Must(template.New("vnc.html").ParseFiles(filepath.Join(tmplPath, "vnc.html")))
		tmpl.Execute(w, config)
	})

	// GET /experiments/{exp}/vms/{name}/vnc/ws
	router.HandleFunc("/experiments/{exp}/vms/{name}/vnc/ws", func(w http.ResponseWriter, r *http.Request) {
		plog.Debug("HTTP handler called", "handler", "GetVNCWebSocket")

		var (
			vars = mux.Vars(r)
			exp  = vars["exp"]
			name = vars["name"]
		)

		endpoint, err := mm.GetVNCEndpoint(mm.NS(exp), mm.VMName(name))
		if err != nil {
			plog.Error("getting VNC endpoint", "err", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		websocket.Handler(util.ConnectWSHandler(endpoint)).ServeHTTP(w, r)
	})

	port := os.Getenv("PHENIX_UGLY_UI_PORT")
	if port == "" {
		port = ":3001"
	}

	plog.Debug("UI config", "assets", assetPath, "templates", tmplPath, "base URL", basePath, "port", port)

	if err := http.ListenAndServe(port, router); err != nil {
		fmt.Printf("ERROR: %v\n", err)
	}
}
