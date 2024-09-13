package main

import (
	"fmt"
	"net/http"
	"os"

	"phenix/api/experiment"
	"phenix/api/vm"
	"phenix/store"
	"phenix/util/plog"

	"github.com/gorilla/mux"
)

var basePath string

func main() {
	plog.NewPhenixHandler()
	plog.SetLevelText("debug")

	switch len(os.Args) {
	case 2:
		tmplPath = os.Args[1]
	case 3:
		tmplPath = os.Args[1]
		basePath = os.Args[2]
	}

	plog.Debug("UI config", "templates", tmplPath, "base URL", basePath, "port", 3001)

	store.Init(store.Endpoint("bolt:///etc/phenix/store.bdb"))

	router := mux.NewRouter().StrictSlash(true)

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, basePath+"/", http.StatusSeeOther)
	})

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var (
			vmFilter       = r.FormValue("vm-filter")
			params         = NewParams(r.FormValue("experiment"))
			experiments, _ = experiment.List()
		)

		plog.Debug("GET index", "exp", r.FormValue("experiment"), "vm-filter", vmFilter)

		for _, exp := range experiments {
			params.Experiments = append(params.Experiments, exp.Metadata.Name)

			if params.Experiment == exp.Metadata.Name {
				params.Topology = exp.Metadata.Annotations["topology"]
				params.Scenario = exp.Metadata.Annotations["scenario"]

				vms, _ := vm.List(exp.Metadata.Name)

				params.VMs = filterVMs(vms, vmFilter)
				params.VMFilter = vmFilter
			}
		}

		Index(w, params)
	})

	router.HandleFunc("/experiments/{name}", func(w http.ResponseWriter, r *http.Request) {
		var (
			exp    = mux.Vars(r)["name"]
			action = r.FormValue("action")

			vms []string
		)

		if r.Form.Has("vms") {
			vms = r.Form["vms"]
		}

		switch action {
		case "boot":
			for _, name := range vms {
				vm.Update(vm.UpdateExperiment(exp), vm.UpdateVM(name), vm.UpdateWithDNB(false))
			}
		case "dnb":
			for _, name := range vms {
				vm.Update(vm.UpdateExperiment(exp), vm.UpdateVM(name), vm.UpdateWithDNB(true))
			}
		}

		http.Redirect(w, r, fmt.Sprintf("%s/?experiment=%s&vm-filter=%s", basePath, exp, r.FormValue("vm-filter")), http.StatusSeeOther)
	})

	if err := http.ListenAndServe(":3001", router); err != nil {
		fmt.Printf("ERROR: %v\n", err)
	}
}
