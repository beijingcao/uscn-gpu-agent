package hardwareinventory

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"syscall"
)

func read(path string) string { b, _ := os.ReadFile(path); return strings.TrimSpace(string(b)) }
func collectPlatform(ctx context.Context, s *Snapshot) {
	s.OS = "Linux"
	s.Kernel = read("/proc/sys/kernel/osrelease")
	s.Environment = "BARE_METAL"
	for _, line := range strings.Split(read("/etc/os-release"), "\n") {
		k, v, ok := strings.Cut(line, "=")
		v = strings.Trim(v, "\"")
		if ok {
			if k == "NAME" {
				s.OS = v
			}
			if k == "VERSION_ID" {
				s.OSVersion = v
			}
		}
	}
	if strings.Contains(strings.ToLower(s.Kernel), "microsoft") {
		s.Environment = "WSL"
	} else if _, e := os.Stat("/.dockerenv"); e == nil {
		s.Environment = "CONTAINER"
	} else if strings.Contains(read("/proc/1/cgroup"), "container") {
		s.Environment = "CONTAINER"
	}
	cpu := read("/proc/cpuinfo")
	sockets := map[string]bool{}
	cores := map[string]bool{}
	physical := ""
	for _, line := range strings.Split(cpu, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "model name":
			if s.CPUModel == "" {
				s.CPUModel = v
			}
		case "physical id":
			physical = v
			sockets[v] = true
		case "core id":
			cores[physical+":"+v] = true
		case "processor":
			s.CPUThreads++
		case "flags":
			if strings.Contains(" "+v+" ", " hypervisor ") && s.Environment == "BARE_METAL" {
				s.Environment = "VIRTUAL_MACHINE"
			}
		}
	}
	s.CPUSockets = int64(len(sockets))
	s.CPUCores = int64(len(cores))
	for _, line := range strings.Split(read("/proc/meminfo"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			s.MemoryBytes = number(fields[1]) * 1024
		case "MemAvailable:":
			s.MemoryAvailableBytes = number(fields[1]) * 1024
		}
	}
	id := read("/sys/class/dmi/id/product_uuid")
	if id == "" {
		id = read("/etc/machine-id")
	}
	if id != "" {
		s.MachineHash = Hash(id)
	}
	s.BootID = read("/proc/sys/kernel/random/boot_id")
	var fs syscall.Statfs_t
	if syscall.Statfs("/", &fs) == nil {
		s.StorageBytes = int64(fs.Blocks) * fs.Bsize
		s.StorageAvailableBytes = int64(fs.Bavail) * fs.Bsize
	}
	b, e := command(ctx, "lsblk", "--json", "--bytes", "--nodeps", "--output", "MODEL,SIZE,TYPE,ROTA")
	if e == nil {
		var v struct {
			Devices []struct {
				Model      string      `json:"model"`
				Size       json.Number `json:"size"`
				Type       string      `json:"type"`
				Rotational bool        `json:"rota"`
			} `json:"blockdevices"`
		}
		if json.Unmarshal(b, &v) == nil {
			for _, d := range v.Devices {
				if d.Type != "disk" {
					continue
				}
				kind := "SSD_OR_VIRTUAL"
				if d.Rotational {
					kind = "HDD"
				}
				s.Disks = append(s.Disks, Disk{Model: strings.TrimSpace(d.Model), Kind: kind, CapacityBytes: number(d.Size.String())})
			}
		}
	}
	// Installed DIMM capacity cannot be inferred from MemTotal. Leave it unknown
	// unless a later explicit, read-only platform adapter can read the modules.
}
