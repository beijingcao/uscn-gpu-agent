package hardwareinventory

import (
	"strings"
	"testing"
	"time"
)

func TestNVIDIAParserRejectsDuplicateAndPreservesModelSuffix(t *testing.T) {
	raw := "GPU-one, NVIDIA GeForce RTX 4090 D, 24564, 570.1, 32, 1024, N/A, N/A, None, N/A\n"
	gs, e := ParseNVIDIA([]byte(raw))
	if e != nil || len(gs) != 1 || gs[0].Model != "NVIDIA GeForce RTX 4090 D" || gs[0].Temperature != nil || gs[0].MemoryUsed == nil || *gs[0].MemoryUsed != 1<<30 || gs[0].PartitionState != "PHYSICAL" {
		t.Fatal("invalid optional sensor/model interpretation", e)
	}
	if _, e = ParseNVIDIA([]byte(raw + raw)); e == nil {
		t.Fatal("duplicate physical UUID accepted")
	}
	for _, raw := range []string{"GPU-one,x,-1,1\n", "MIG-one,x,100,1\n", strings.Repeat("x", 100)} {
		if _, e = ParseNVIDIA([]byte(raw)); e == nil {
			t.Fatal("malformed discovery accepted")
		}
	}
	gs, e = ParseNVIDIA([]byte("GPU-one,A100,81920,570,0,0,30,50,None,Enabled\n"))
	if e != nil || !gs[0].Virtual || gs[0].PartitionState != "PARTITIONED" {
		t.Fatal("MIG treated as whole available card")
	}
}
func TestConfigurationDigestSeparatesMeasurementsFromHardware(t *testing.T) {
	s := Snapshot{Schema: 1, CapturedAt: time.Now(), MachineHash: Hash("fixture"), BootID: "boot", OS: "Linux", Environment: "BARE_METAL", GPUs: []GPU{{ID: "GPU-one", Model: "RTX 4090", Vendor: "NVIDIA", MemoryBytes: 24 << 30, PartitionState: "PHYSICAL"}}}
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
	h := s.ConfigurationHash()
	s.CapturedAt = s.CapturedAt.Add(time.Minute)
	s.BootID = "new-boot"
	s.MemoryAvailableBytes = 2
	s.StorageAvailableBytes = 3
	temp := int64(60)
	s.GPUs[0].Temperature = &temp
	if h != s.ConfigurationHash() {
		t.Fatal("dynamic usage changed approved hardware identity")
	}
	s.GPUs[0].Model = "RTX 4090 D"
	if h == s.ConfigurationHash() {
		t.Fatal("different model suffix silently accepted")
	}
	s.GPUs[0].Model = "bad\nmodel"
	if s.Validate() == nil {
		t.Fatal("control characters accepted")
	}
}
