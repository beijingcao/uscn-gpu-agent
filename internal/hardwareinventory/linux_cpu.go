package hardwareinventory

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// linuxCPU supplements ARM /proc/cpuinfo, which commonly omits x86 socket/core
// fields. Only actual per-CPU topology is counted; unknown topology stays zero.
func linuxCPU(cpuinfo string, lscpu []byte, readFile func(string) string) (string, int64, int64) {
	model := ""
	var data struct {
		CPUs []struct {
			Field string `json:"field"`
			Data  string `json:"data"`
		} `json:"lscpu"`
	}
	if json.Unmarshal(lscpu, &data) == nil {
		for _, row := range data.CPUs {
			if strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(row.Field, ":")), "Model name") && strings.TrimSpace(row.Data) != "" {
				model = strings.TrimSpace(row.Data)
				break
			}
		}
	}
	sockets, cores, parts := map[string]bool{}, map[string]bool{}, map[string]bool{}
	complete := true
	processors := 0
	for _, block := range strings.Split(strings.TrimSpace(cpuinfo), "\n\n") {
		values := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			if k, v, ok := strings.Cut(line, ":"); ok {
				values[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		id, ok := values["processor"]
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			complete = false
			continue
		}
		processors++
		base := "/sys/devices/system/cpu/cpu" + strconv.FormatUint(n, 10) + "/topology/"
		socket, core := readFile(base+"physical_package_id"), readFile(base+"core_id")
		a, ea := strconv.ParseInt(socket, 10, 32)
		b, eb := strconv.ParseInt(core, 10, 32)
		if ea != nil || eb != nil || a < 0 || b < 0 {
			complete = false
		} else {
			sockets[socket] = true
			cores[socket+":"+core] = true
		}
		if values["CPU implementer"] != "" && values["CPU part"] != "" {
			parts["ARM implementer "+values["CPU implementer"]+" part "+values["CPU part"]] = true
		}
	}
	if model == "" && len(parts) > 0 {
		names := []string{}
		for p := range parts {
			names = append(names, p)
		}
		sort.Strings(names)
		model = strings.Join(names, "; ")
	}
	if !complete || processors == 0 {
		return model, 0, 0
	}
	return model, int64(len(sockets)), int64(len(cores))
}
