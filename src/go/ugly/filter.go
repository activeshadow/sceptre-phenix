package main

import (
	"phenix/util/mm"
	"phenix/util/plog"
	"strings"
)

func filterVMs(vms []mm.VM, filter string) []VM {
	var filtered []VM
	filter = strings.TrimSpace(filter)

	if len(filter) == 0 {
		for _, vm := range vms {
			filtered = append(filtered, VM{Name: vm.Name, DNB: vm.DoNotBoot})
		}

		return filtered
	}

	var (
		fields = strings.Fields(filter)
		labels = make(map[string]string)
		terms  []string
	)

	for _, field := range fields {
		plog.Debug("filter setup", "field", field)

		if strings.Contains(field, "label:") {
			label := strings.TrimPrefix(field, "label:")
			kv := strings.Split(label, "=")

			plog.Debug("filter label", "label", label, "kv", kv)

			if len(kv) == 1 {
				continue
			} else {
				labels[kv[0]] = kv[1]
			}
		} else {
			terms = append(terms, field)
		}
	}

	plog.Debug("filter labels", "labels", labels)

	for _, vm := range vms {
		plog.Debug("VM details", "name", vm.Name, "labels", vm.Labels)

		keep := false

		for k, v := range labels {
			if val, ok := vm.Labels[k]; ok {
				keep = keep || (v == val || v == "*")
			}
		}

		for _, t := range terms {
			keep = keep || strings.Contains(vm.Name, t)
		}

		if keep {
			filtered = append(filtered, VM{Name: vm.Name, DNB: vm.DoNotBoot})
		}
	}

	return filtered
}
