package hardwareinventory

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func command(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if e := cmd.Run(); e != nil {
		return nil, errors.New("metadata query unavailable")
	}
	return out.Bytes(), nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 8<<20 {
		return 0, errors.New("metadata exceeds limit")
	}
	return b.Buffer.Write(p)
}
func value(b []byte) string { return strings.TrimSpace(string(b)) }
func number(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if v < 0 {
		return 0
	}
	return v
}
func pointer(s string) *int64 {
	v, e := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if e != nil || v < 0 {
		return nil
	}
	return &v
}

// IsGB10 matches the driver's exact GB10 model, never an arbitrary N/A device.
func IsGB10(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "NVIDIA GB10") || strings.EqualFold(strings.TrimSpace(model), "GB10")
}
func unsupported(value string) bool {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(value), "[]")) {
	case "n/a", "not supported":
		return true
	default:
		return false
	}
}
func ParseNVIDIA(b []byte) ([]GPU, error) {
	r := csv.NewReader(bytes.NewReader(b))
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	out := []GPU{}
	seen := map[string]bool{}
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil || len(row) < 4 {
			return nil, errors.New("invalid NVIDIA metadata")
		}
		id := strings.TrimSpace(row[0])
		mem := number(row[2])
		unified := IsGB10(strings.TrimSpace(row[1])) && unsupported(row[2])
		if !strings.HasPrefix(id, "GPU-") || seen[id] || (!unified && mem <= 0) || mem > 1048576 {
			return nil, errors.New("invalid NVIDIA identity")
		}
		seen[id] = true
		g := GPU{ID: id, Vendor: "NVIDIA", Model: strings.TrimSpace(row[1]), MemoryBytes: mem << 20, UnifiedMemory: unified, Driver: strings.TrimSpace(row[3]), PartitionState: "UNKNOWN"}
		if IsGB10(g.Model) {
			g.UnifiedMemory = true
			g.MemoryBytes = 0
		}
		if len(row) >= 8 {
			g.Utilization = pointer(row[4])
			g.MemoryUsed = pointer(row[5])
			if g.MemoryUsed != nil {
				*g.MemoryUsed *= 1 << 20
			}
			g.Temperature = pointer(row[6])
			if v, e := strconv.ParseFloat(strings.TrimSpace(row[7]), 64); e == nil && v >= 0 && v <= 10000 {
				n := int64(v * 1000)
				g.PowerMilliwatts = &n
			}
		}
		if len(row) >= 10 {
			g.Virtual = strings.Contains(strings.ToLower(row[8]), "vgpu") || strings.EqualFold(strings.TrimSpace(row[9]), "Enabled")
			if g.Virtual {
				g.PartitionState = "PARTITIONED"
			} else if strings.EqualFold(strings.TrimSpace(row[8]), "None") && (strings.EqualFold(strings.TrimSpace(row[9]), "Disabled") || unsupported(row[9])) {
				g.PartitionState = "PHYSICAL"
			}
		}
		out = append(out, g)
		if len(out) > 128 {
			return nil, errors.New("too many devices")
		}
	}
	return out, nil
}
func nvidia(ctx context.Context) ([]GPU, error) {
	b, e := command(ctx, "nvidia-smi", "--query-gpu=uuid,name,memory.total,driver_version,utilization.gpu,memory.used,temperature.gpu,power.draw,virtualization.mode,mig.mode.current", "--format=csv,noheader,nounits")
	if e != nil {
		b, e = command(ctx, "nvidia-smi", "--query-gpu=uuid,name,memory.total,driver_version", "--format=csv,noheader,nounits")
	}
	if e != nil {
		return nil, e
	}
	gs, e := ParseNVIDIA(b)
	if e != nil {
		return nil, e
	}
	// Driver releases differ in their CSV query fields. The documented XML
	// report independently identifies MIG/vGPU mode; unknown modes fail closed.
	raw, err := command(ctx, "nvidia-smi", "-q", "-x")
	if err == nil {
		var doc struct {
			GPUs []struct {
				UUID           string `xml:"uuid"`
				Virtualization string `xml:"gpu_virtualization_mode>virtualization_mode"`
				MIG            string `xml:"mig_mode>current_mig"`
			} `xml:"gpu"`
		}
		if xml.Unmarshal(raw, &doc) == nil {
			for i := range gs {
				for _, g := range doc.GPUs {
					if g.UUID != gs[i].ID {
						continue
					}
					if strings.EqualFold(strings.TrimSpace(g.MIG), "Enabled") || strings.Contains(strings.ToLower(g.Virtualization), "vgpu") {
						gs[i].Virtual = true
						gs[i].PartitionState = "PARTITIONED"
					} else if strings.EqualFold(strings.TrimSpace(g.Virtualization), "None") && (strings.EqualFold(strings.TrimSpace(g.MIG), "Disabled") || unsupported(g.MIG)) {
						gs[i].PartitionState = "PHYSICAL"
					}
				}
			}
		}
	}
	return gs, nil
}
func Collect(ctx context.Context) (Snapshot, error) {
	s := Snapshot{Schema: Schema, CapturedAt: time.Now().UTC(), Architecture: runtime.GOARCH, Environment: "UNKNOWN", GPUs: []GPU{}, Disks: []Disk{}, NetworkModels: []string{}, Problems: []string{}}
	collectPlatform(ctx, &s)
	if gs, e := nvidia(ctx); e == nil {
		s.GPUs = append(s.GPUs, gs...)
	} else {
		s.Problems = append(s.Problems, "NVIDIA_DRIVER_OR_DEVICE_UNAVAILABLE")
	}
	if s.MachineHash == "" {
		s.Problems = append(s.Problems, "MACHINE_ID_UNAVAILABLE")
		return s, errors.New("stable machine identity unavailable")
	}
	if s.BootID == "" {
		return s, errors.New("boot identity unavailable")
	}
	if s.CPUModel == "" {
		s.Problems = append(s.Problems, "CPU_MODEL_UNAVAILABLE")
	}
	if s.CPUSockets == 0 {
		s.Problems = append(s.Problems, "CPU_SOCKET_COUNT_UNAVAILABLE")
	}
	if s.MemoryBytes == 0 {
		s.Problems = append(s.Problems, "MEMORY_UNAVAILABLE")
	}
	if s.StorageBytes == 0 {
		s.Problems = append(s.Problems, "STORAGE_UNAVAILABLE")
	}
	return s, s.Validate()
}
