package hardwareinventory

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"syscall"
)

func collectPlatform(ctx context.Context, s *Snapshot) {
	query := func(key string) string { b, _ := command(ctx, "/usr/sbin/sysctl", "-n", key); return value(b) }
	s.OS = "macOS"
	b, _ := command(ctx, "/usr/bin/sw_vers", "-productVersion")
	s.OSVersion = value(b)
	s.Kernel = query("kern.osrelease")
	s.Environment = "BARE_METAL"
	if query("kern.hv_vmm_present") == "1" {
		s.Environment = "VIRTUAL_MACHINE"
	}
	s.CPUModel = query("machdep.cpu.brand_string")
	s.CPUSockets = number(query("hw.packages"))
	s.CPUCores = number(query("hw.physicalcpu"))
	s.CPUThreads = number(query("hw.logicalcpu"))
	s.MemoryBytes = number(query("hw.memsize"))
	s.InstalledMemoryBytes = s.MemoryBytes
	if boot := query("kern.boottime"); boot != "" {
		s.BootID = Hash(boot)
	}
	b, _ = command(ctx, "/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	if m := regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`).FindSubmatch(b); len(m) == 2 {
		s.MachineHash = Hash(string(m[1]))
	}
	var fs syscall.Statfs_t
	if syscall.Statfs("/", &fs) == nil {
		s.StorageBytes = int64(fs.Blocks) * int64(fs.Bsize)
		s.StorageAvailableBytes = int64(fs.Bavail) * int64(fs.Bsize)
	}
	b, _ = command(ctx, "/usr/bin/vm_stat")
	text := string(b)
	page := int64(4096)
	if m := regexp.MustCompile(`page size of (\d+) bytes`).FindStringSubmatch(text); len(m) == 2 {
		page = number(m[1])
	}
	for _, key := range []string{"Pages free", "Pages inactive", "Pages speculative"} {
		m := regexp.MustCompile(`(?m)^` + key + `:\s+(\d+)\.`).FindStringSubmatch(text)
		if len(m) == 2 {
			s.MemoryAvailableBytes += number(m[1]) * page
		}
	}
	if s.MemoryAvailableBytes > s.MemoryBytes {
		s.MemoryAvailableBytes = s.MemoryBytes
	}
	b, e := command(ctx, "/usr/sbin/system_profiler", "-json", "SPHardwareDataType", "SPDisplaysDataType", "SPStorageDataType")
	if e == nil {
		var v struct {
			Hardware []map[string]any `json:"SPHardwareDataType"`
			Displays []map[string]any `json:"SPDisplaysDataType"`
			Storage  []map[string]any `json:"SPStorageDataType"`
		}
		if json.Unmarshal(b, &v) == nil {
			get := func(m map[string]any, k string) string { x, _ := m[k].(string); return x }
			if s.CPUModel == "" && len(v.Hardware) > 0 {
				s.CPUModel = get(v.Hardware[0], "chip_type")
				if s.CPUModel == "" {
					s.CPUModel = get(v.Hardware[0], "cpu_type")
				}
			}
			for i, d := range v.Displays {
				model := get(d, "sppci_model")
				if model == "" {
					model = get(d, "_name")
				}
				vendor := strings.ToLower(get(d, "spdisplays_vendor") + " " + model)
				g := GPU{ID: Hash(s.MachineHash + ":" + model + ":" + string(rune('0'+i))), Vendor: "OTHER", Model: model}
				switch {
				case strings.Contains(vendor, "apple"):
					g.Vendor = "APPLE"
					g.UnifiedMemory = s.Architecture == "arm64"
				case strings.Contains(vendor, "amd") || strings.Contains(vendor, "radeon"):
					g.Vendor = "AMD"
				case strings.Contains(vendor, "intel"):
					g.Vendor = "INTEL"
				case strings.Contains(vendor, "nvidia"):
					g.Vendor = "NVIDIA"
				}
				// A legacy macOS display entry is not an NVML-verified NVIDIA device.
				if g.Vendor != "NVIDIA" && g.Model != "" {
					s.GPUs = append(s.GPUs, g)
				}
			}
			for _, d := range v.Storage {
				if physical, ok := d["physical_drive"].(map[string]any); ok {
					model := get(physical, "device_name")
					kind := get(physical, "medium_type")
					if model != "" {
						s.Disks = append(s.Disks, Disk{Model: model, Kind: kind})
					}
				}
			}
		}
	}
}
