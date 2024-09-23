package main

import (
	"strconv"
	"strings"

	"phenix/util/mm"
	"phenix/util/plog"
)

func convert(vm mm.VM) VM {
	return VM{
		Name: vm.Name,
		DNB:  vm.DoNotBoot,
	}
}

func filterVMs(vms []mm.VM, filter string) []VM {
	var filtered []VM
	filter = strings.TrimSpace(filter)

	if len(filter) == 0 {
		for _, vm := range vms {
			filtered = append(filtered, convert(vm))
		}

		return filtered
	}

	var (
		fields = strings.Fields(filter)
		labels = make(map[string]string)
		dnb    *bool
		terms  []string
	)

	for _, field := range fields {
		plog.Debug("filter setup", "field", field)

		if strings.Contains(field, "label:") {
			label := strings.TrimPrefix(field, "label:")
			kv := strings.Split(label, "=")

			plog.Debug("filter label", "field", label, "label", kv)

			if len(kv) == 1 {
				continue
			} else {
				labels[kv[0]] = kv[1]
			}
		} else if strings.Contains(field, "dnb:") {
			str := strings.TrimPrefix(field, "dnb:")
			val, _ := strconv.ParseBool(str)

			plog.Debug("filter dnb", "field", str, "dnb", val)

			dnb = &val
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

		if dnb != nil {
			keep = keep || vm.DoNotBoot == *dnb
		}

		for _, t := range terms {
			keep = keep || strings.Contains(vm.Name, t)
		}

		if keep {
			filtered = append(filtered, convert(vm))
		}
	}

	return filtered
}
