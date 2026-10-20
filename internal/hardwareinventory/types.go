// Package hardwareinventory collects read-only host metadata, never workloads,
// process arguments, credentials, files, or user directory contents.
package hardwareinventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
)

const Schema = 1
const MaxBytes int64 = 1 << 50

type GPU struct {
	ID              string `json:"id"`
	Vendor          string `json:"vendor"`
	Model           string `json:"model"`
	MemoryBytes     int64  `json:"memory_bytes"`
	UnifiedMemory   bool   `json:"unified_memory"`
	Driver          string `json:"driver"`
	Virtual         bool   `json:"virtual"`
	PartitionState  string `json:"partition_state"`
	Utilization     *int64 `json:"utilization_percent,omitempty"`
	MemoryUsed      *int64 `json:"memory_used_bytes,omitempty"`
	Temperature     *int64 `json:"temperature_c,omitempty"`
	PowerMilliwatts *int64 `json:"power_milliwatts,omitempty"`
}
type Disk struct {
	Model         string `json:"model"`
	Kind          string `json:"kind"`
	CapacityBytes int64  `json:"capacity_bytes"`
}
type Snapshot struct {
	Schema                int       `json:"schema"`
	CapturedAt            time.Time `json:"captured_at"`
	MachineHash           string    `json:"machine_hash"`
	BootID                string    `json:"boot_id"`
	OS                    string    `json:"os"`
	OSVersion             string    `json:"os_version"`
	Kernel                string    `json:"kernel"`
	Architecture          string    `json:"architecture"`
	Environment           string    `json:"environment"`
	CPUModel              string    `json:"cpu_model"`
	CPUSockets            int64     `json:"cpu_sockets"`
	CPUCores              int64     `json:"cpu_cores"`
	CPUThreads            int64     `json:"cpu_threads"`
	InstalledMemoryBytes  int64     `json:"installed_memory_bytes"`
	MemoryBytes           int64     `json:"memory_bytes"`
	MemoryAvailableBytes  int64     `json:"memory_available_bytes"`
	StorageBytes          int64     `json:"storage_bytes"`
	StorageAvailableBytes int64     `json:"storage_available_bytes"`
	GPUs                  []GPU     `json:"gpus"`
	Disks                 []Disk    `json:"disks"`
	NetworkModels         []string  `json:"network_models"`
	Problems              []string  `json:"problems"`
}

func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func safe(s string, max int) bool {
	return len(s) <= max && strings.IndexFunc(s, unicode.IsControl) < 0
}
func (s Snapshot) Validate() error {
	bad := func() error { return errors.New("invalid hardware metadata") }
	if s.Schema != Schema || s.CapturedAt.IsZero() || len(s.GPUs) > 128 || len(s.Disks) > 256 || len(s.NetworkModels) > 64 || len(s.Problems) > 64 {
		return bad()
	}
	if len(s.MachineHash) != 64 || len(s.BootID) > 128 || s.BootID == "" {
		return bad()
	}
	if _, e := hex.DecodeString(s.MachineHash); e != nil {
		return bad()
	}
	for _, v := range []string{s.OS, s.OSVersion, s.Kernel, s.Architecture, s.CPUModel, s.BootID} {
		if !safe(v, 256) {
			return bad()
		}
	}
	switch s.Environment {
	case "BARE_METAL", "VIRTUAL_MACHINE", "CONTAINER", "WSL", "UNKNOWN":
	default:
		return bad()
	}
	for _, v := range []int64{s.CPUSockets, s.CPUCores, s.CPUThreads, s.InstalledMemoryBytes, s.MemoryBytes, s.MemoryAvailableBytes, s.StorageBytes, s.StorageAvailableBytes} {
		if v < 0 || v > MaxBytes {
			return bad()
		}
	}
	if s.MemoryAvailableBytes > s.MemoryBytes || s.StorageAvailableBytes > s.StorageBytes || s.CPUSockets > 1024 || s.CPUCores > 65536 || s.CPUThreads > 131072 {
		return bad()
	}
	seen := map[string]bool{}
	for _, g := range s.GPUs {
		if g.ID == "" || seen[g.ID] || !safe(g.ID, 256) || !safe(g.Model, 256) || !safe(g.Driver, 128) || g.Model == "" || g.MemoryBytes < 0 || g.MemoryBytes > MaxBytes {
			return bad()
		}
		seen[g.ID] = true
		switch g.Vendor {
		case "NVIDIA", "APPLE", "AMD", "INTEL", "OTHER":
		default:
			return bad()
		}
		if g.Utilization != nil && (*g.Utilization < 0 || *g.Utilization > 100) {
			return bad()
		}
		if g.MemoryUsed != nil && (*g.MemoryUsed < 0 || *g.MemoryUsed > MaxBytes) {
			return bad()
		}
		if g.Temperature != nil && (*g.Temperature < -30 || *g.Temperature > 200) {
			return bad()
		}
		if g.PowerMilliwatts != nil && (*g.PowerMilliwatts < 0 || *g.PowerMilliwatts > 10000000) {
			return bad()
		}
	}
	for _, d := range s.Disks {
		if !safe(d.Model, 256) || !safe(d.Kind, 64) || d.CapacityBytes < 0 || d.CapacityBytes > MaxBytes {
			return bad()
		}
	}
	for _, v := range append(append([]string{}, s.NetworkModels...), s.Problems...) {
		if !safe(v, 256) {
			return bad()
		}
	}
	return nil
}

// ConfigurationHash excludes dynamic measurements and sample time. A heartbeat
// or lower free-memory reading must not silently rewrite an approved contract.
func (s Snapshot) ConfigurationHash() string {
	s.CapturedAt = time.Time{}
	s.BootID = ""
	s.MemoryAvailableBytes = 0
	s.StorageAvailableBytes = 0
	s.Problems = nil
	s.GPUs = append([]GPU{}, s.GPUs...)
	for i := range s.GPUs {
		s.GPUs[i].Utilization = nil
		s.GPUs[i].MemoryUsed = nil
		s.GPUs[i].Temperature = nil
		s.GPUs[i].PowerMilliwatts = nil
	}
	b, _ := json.Marshal(s)
	return Hash(string(b))
}
