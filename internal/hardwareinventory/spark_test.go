package hardwareinventory

import (
	"fmt"
	"strings"
	"testing"
)

func TestGB10UnifiedMemoryNADoesNotInventVRAM(t *testing.T) {
	for _, memory := range []string{"N/A", "[N/A]", "[Not Supported]", "128000"} {
		g, err := ParseNVIDIA([]byte("GPU-spark,NVIDIA GB10," + memory + ",580.159.03,0,[N/A],42,12.50,None,N/A\n"))
		if err != nil || len(g) != 1 {
			t.Fatal(memory, err)
		}
		if !g[0].UnifiedMemory || g[0].MemoryBytes != 0 || g[0].MemoryUsed != nil || g[0].PartitionState != "PHYSICAL" {
			t.Fatalf("false dedicated VRAM: %+v", g[0])
		}
	}
	for _, row := range []string{"GPU-a,RTX 4090,N/A,580\n", "GPU-a,NVIDIA GB10,garbage,580\n", "GPU-a,NVIDIA GB10,-1,580\n", "GPU-a,NVIDIA GB100,N/A,580\n"} {
		if _, err := ParseNVIDIA([]byte(row)); err == nil {
			t.Fatal("accepted invalid/unknown memory", row)
		}
	}
	g, err := ParseNVIDIA([]byte("GPU-spark,NVIDIA GB10,N/A,580,0,N/A,42,12,None,Enabled\n"))
	if err != nil || !g[0].Virtual || g[0].PartitionState != "PARTITIONED" {
		t.Fatal("MIG guard bypassed", err)
	}
	g, err = ParseNVIDIA([]byte("GPU-spark,NVIDIA GB10,N/A,580\n"))
	if err != nil || g[0].PartitionState != "UNKNOWN" {
		t.Fatal("unconfirmed physical identity", err)
	}
}
func TestLinuxARMBigLittleTopologyAndMissingMetadata(t *testing.T) {
	var cpu strings.Builder
	files := map[string]string{}
	for i := 0; i < 20; i++ {
		part := "0xd85"
		if i >= 10 {
			part = "0xd87"
		}
		fmt.Fprintf(&cpu, "processor : %d\nCPU implementer : 0x41\nCPU part : %s\n\n", i, part)
		base := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/topology/", i)
		files[base+"physical_package_id"] = "0"
		files[base+"core_id"] = fmt.Sprint(i)
	}
	read := func(p string) string { return files[p] }
	model, sockets, cores := linuxCPU(cpu.String(), []byte(`{"lscpu":[{"field":"Model name:","data":"Cortex-X925"}]}`), read)
	if model != "Cortex-X925" || sockets != 1 || cores != 20 {
		t.Fatal(model, sockets, cores)
	}
	model, sockets, cores = linuxCPU(cpu.String(), nil, read)
	if !strings.Contains(model, "0xd85") || !strings.Contains(model, "0xd87") || sockets != 1 || cores != 20 {
		t.Fatal("heterogeneous ARM identifiers lost", model, sockets, cores)
	}
	delete(files, "/sys/devices/system/cpu/cpu19/topology/physical_package_id")
	_, sockets, cores = linuxCPU(cpu.String(), nil, read)
	if sockets != 0 || cores != 0 {
		t.Fatal("unknown topology invented")
	}
}
