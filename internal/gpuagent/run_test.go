package gpuagent

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"github.com/beijingcao/uscn-gpu-agent/internal/deviceprotocol"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScheduleKeepsPhaseAndExposesRestartGaps(t *testing.T) {
	now := time.Unix(1250, 0)
	next, missed := AdvanceSchedule(0, now)
	if next != 1800 || missed != 0 {
		t.Fatal(next, missed)
	}
	next, missed = AdvanceSchedule(1800, time.Unix(3050, 0))
	if next != 3600 || missed != 2 {
		t.Fatal("restart reset or backfilled schedule", next, missed)
	}
	next, missed = AdvanceSchedule(3600, time.Unix(3100, 0))
	if next != 3600 || missed != 0 {
		t.Fatal("unnecessary schedule reset")
	}
}
func TestConfigQueueAndServerAcknowledgement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "device.json")
	c, e := NewConfig("https://uscn.si")
	if e != nil {
		t.Fatal(e)
	}
	c.DeviceID = "00000000-0000-4000-8000-000000000001"
	c.NodeID = "00000000-0000-4000-8000-000000000002"
	c.Epoch = 1
	if e = Save(path, c); e != nil {
		t.Fatal(e)
	}
	r, e := report(path, &c, "HEARTBEAT", nil)
	if e != nil {
		t.Fatal(e)
	}
	saved, e := Load(path)
	if e != nil || saved.Sequence != 1 {
		t.Fatal("sequence not durable before send")
	}
	key, _ := c.Key()
	if r.Verify(key.Public().(ed25519.PublicKey)) != nil {
		t.Fatal("queued report signature invalid")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var in deviceprotocol.Report
		_ = json.NewDecoder(req.Body).Decode(&in)
		_ = json.NewEncoder(w).Encode(deviceprotocol.Ack{ID: in.ID, Fresh: true, ReceivedAt: time.Now(), ReportSeconds: 600, ProbeNonce: "probe"})
	}))
	defer server.Close()
	c.Platform = server.URL
	if _, e = send(context.Background(), &c, r); e != nil {
		t.Fatal(e)
	}
	r.Kind = "STOPPED"
	r.Sign(key)
	outbox := filepath.Join(dir, "outbox")
	if e = enqueue(outbox, r); e != nil {
		t.Fatal(e)
	}
	if e = flush(context.Background(), &c, outbox); e != nil {
		t.Fatal(e)
	}
	files, _ := os.ReadDir(outbox)
	if len(files) != 0 {
		t.Fatal("acknowledged report retained for repeat posting")
	}
}
