package hardwareinventory

import (
	"context"
	"encoding/json"
	"strings"
)

// Fixed local CIM query: no caller-supplied script, hostname, or business data.
const windowsQuery = `$ErrorActionPreference='Stop';$o=Get-CimInstance Win32_OperatingSystem;$c=@(Get-CimInstance Win32_Processor);$s=Get-CimInstance Win32_ComputerSystem;$u=Get-CimInstance Win32_ComputerSystemProduct;$d=Get-CimInstance Win32_LogicalDisk -Filter "DeviceID='$($env:SystemDrive)'";[pscustomobject]@{os=$o.Caption;version=$o.Version;build=$o.BuildNumber;boot=$o.LastBootUpTime.ToUniversalTime().ToString('o');uuid=$u.UUID;model=$s.Model;manufacturer=$s.Manufacturer;cpu=@($c|ForEach-Object{[pscustomobject]@{model=$_.Name;cores=$_.NumberOfCores;threads=$_.NumberOfLogicalProcessors}});memory=[long]$o.TotalVisibleMemorySize*1024;installed=[long]$s.TotalPhysicalMemory;available=[long]$o.FreePhysicalMemory*1024;storage=[long]$d.Size;free=[long]$d.FreeSpace;disks=@(Get-CimInstance Win32_DiskDrive|ForEach-Object{[pscustomobject]@{model=$_.Model;size=[long]$_.Size;kind=$_.MediaType}});network=@(Get-CimInstance Win32_NetworkAdapter -Filter 'PhysicalAdapter=True'|ForEach-Object{$_.Name})}|ConvertTo-Json -Depth 5 -Compress`

func collectPlatform(ctx context.Context, s *Snapshot) {
	b, e := command(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", windowsQuery)
	if e != nil {
		s.Problems = append(s.Problems, "WINDOWS_CIM_UNAVAILABLE")
		return
	}
	var v struct {
		OS           string `json:"os"`
		Version      string `json:"version"`
		Build        string `json:"build"`
		Boot         string `json:"boot"`
		UUID         string `json:"uuid"`
		Model        string `json:"model"`
		Manufacturer string `json:"manufacturer"`
		CPUs         []struct {
			Model   string `json:"model"`
			Cores   int64  `json:"cores"`
			Threads int64  `json:"threads"`
		} `json:"cpu"`
		Memory    int64 `json:"memory"`
		Installed int64 `json:"installed"`
		Available int64 `json:"available"`
		Storage   int64 `json:"storage"`
		Free      int64 `json:"free"`
		Disks     []struct {
			Model string `json:"model"`
			Size  int64  `json:"size"`
			Kind  string `json:"kind"`
		} `json:"disks"`
		Network []string `json:"network"`
	}
	if json.Unmarshal(b, &v) != nil {
		s.Problems = append(s.Problems, "WINDOWS_CIM_INVALID")
		return
	}
	s.OS = v.OS
	s.OSVersion = v.Version
	s.Kernel = v.Build
	s.BootID = Hash(v.Boot)
	if v.UUID != "" {
		s.MachineHash = Hash(v.UUID)
	}
	s.Environment = "BARE_METAL"
	m := strings.ToLower(v.Model + " " + v.Manufacturer)
	for _, term := range []string{"virtual", "vmware", "qemu", "kvm", "xen", "parallels"} {
		if strings.Contains(m, term) {
			s.Environment = "VIRTUAL_MACHINE"
		}
	}
	s.CPUSockets = int64(len(v.CPUs))
	for _, c := range v.CPUs {
		if s.CPUModel == "" {
			s.CPUModel = strings.TrimSpace(c.Model)
		}
		s.CPUCores += c.Cores
		s.CPUThreads += c.Threads
	}
	s.MemoryBytes = v.Memory
	s.InstalledMemoryBytes = v.Installed
	s.MemoryAvailableBytes = v.Available
	s.StorageBytes = v.Storage
	s.StorageAvailableBytes = v.Free
	for _, d := range v.Disks {
		s.Disks = append(s.Disks, Disk{Model: d.Model, Kind: d.Kind, CapacityBytes: d.Size})
	}
	s.NetworkModels = v.Network
}
