package gpuagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/beijingcao/uscn-gpu-agent/internal/deviceprotocol"
	"github.com/beijingcao/uscn-gpu-agent/internal/hardwareinventory"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func id() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	v := hex.EncodeToString(b[:])
	return v[:8] + "-" + v[8:12] + "-" + v[12:16] + "-" + v[16:20] + "-" + v[20:]
}

// AdvanceSchedule preserves the established ten-minute phase across restarts.
// Missed periods remain missing; it never fabricates samples for past slots.
func AdvanceSchedule(next int64, now time.Time) (int64, int64) {
	if next == 0 {
		return (now.Unix()/600 + 1) * 600, 0
	}
	if next > now.Unix() {
		return next, 0
	}
	missed := (now.Unix() - next) / 600
	return next + (missed+1)*600, missed
}
func report(path string, c *Config, kind string, s *hardwareinventory.Snapshot) (deviceprotocol.Report, error) {
	c.Sequence++
	if e := Save(path, *c); e != nil {
		return deviceprotocol.Report{}, e
	}
	key, e := c.Key()
	if e != nil {
		return deviceprotocol.Report{}, e
	}
	r := deviceprotocol.Report{Protocol: 1, DeviceID: c.DeviceID, Epoch: c.Epoch, ID: id(), Sequence: c.Sequence, SentAt: time.Now().UTC(), Kind: kind, AgentVersion: Version, ProbeNonce: c.ProbeNonce, GapCount: c.GapCount, Snapshot: s}
	r.Sign(key)
	return r, nil
}
func send(ctx context.Context, c *Config, r deviceprotocol.Report) (deviceprotocol.Ack, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var ack deviceprotocol.Ack
	e := post(ctx, c.Platform, "/api/v1/gpu-agent/reports", r, &ack)
	if e == nil && (ack.ID != r.ID || ack.ReportSeconds != 600) {
		e = errors.New("invalid report acknowledgement")
	}
	return ack, e
}
func enqueue(dir string, r deviceprotocol.Report) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return e
	}
	var size int64
	for _, entry := range entries {
		info, e := entry.Info()
		if e != nil || !info.Mode().IsRegular() {
			return errors.New("invalid outbox")
		}
		size += info.Size()
		if time.Since(info.ModTime()) > 72*time.Hour {
			return errors.New("device evidence buffer exceeded 72 hours")
		}
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if size+int64(len(raw)) > 64<<20 {
		return errors.New("device evidence buffer full")
	}
	f, e := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%016d-%s.json", r.Sequence, r.ID)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(raw)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func flush(ctx context.Context, c *Config, dir string) error {
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for i, entry := range entries {
		if i >= 2 {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return errors.New("invalid outbox entry")
		}
		path := filepath.Join(dir, entry.Name())
		raw, e := os.ReadFile(path)
		if e != nil || len(raw) > deviceprotocol.MaxBody {
			return errors.New("invalid queued report")
		}
		var r deviceprotocol.Report
		if json.Unmarshal(raw, &r) != nil || r.DeviceID != c.DeviceID {
			return errors.New("invalid queued device identity")
		}
		if _, e = send(ctx, c, r); e != nil {
			return e
		}
		if e = os.Remove(path); e != nil {
			return e
		}
	}
	return nil
}

type Options struct {
	Collect func(context.Context) (hardwareinventory.Snapshot, error)
	Status  func(string)
}

func Run(ctx context.Context, path string, opt Options) error {
	c, e := Load(path)
	if e != nil {
		return e
	}
	if c.DeviceID == "" {
		return errors.New("device enrollment required")
	}
	unlock, e := lock(path + ".lock")
	if e != nil {
		return e
	}
	defer unlock()
	if opt.Collect == nil {
		opt.Collect = hardwareinventory.Collect
	}
	status := func(s string) {
		if opt.Status != nil {
			opt.Status(s)
		}
	}
	outbox := filepath.Join(filepath.Dir(path), "outbox")
	lastProbe := ""
	lastConfig := ""
	nextLocalSample := time.Now()
	nextHeartbeat := time.Now()
	initial := true
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		now := time.Now()
		snapshotDue := initial || now.Unix() >= c.NextReportUnix || c.ProbeNonce != lastProbe
		if snapshotDue || !now.Before(nextLocalSample) {
			sampleCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
			snapshot, err := opt.Collect(sampleCtx)
			cancel()
			nextLocalSample = now.Add(time.Minute)
			configuration := ""
			if err == nil {
				configuration = snapshot.ConfigurationHash()
			}
			// Cheap periodic local sampling additionally catches configuration changes;
			// only changes, probes, starts and scheduled reports are transmitted.
			if snapshotDue || configuration != "" && configuration != lastConfig {
				kind := "SNAPSHOT"
				var ptr *hardwareinventory.Snapshot = &snapshot
				if err != nil {
					kind = "COLLECTION_ERROR"
					ptr = nil
					status("COLLECTION_FAILED")
				}
				if now.Unix() >= c.NextReportUnix {
					var missed int64
					c.NextReportUnix, missed = AdvanceSchedule(c.NextReportUnix, now)
					c.GapCount += missed
				}
				r, err := report(path, &c, kind, ptr)
				if err != nil {
					return err
				}
				ack, err := send(ctx, &c, r)
				if err != nil {
					var rejection *Rejected
					if errors.As(err, &rejection) && (rejection.Status == 401 || rejection.Status == 403) {
						return err
					}
					if err = enqueue(outbox, r); err != nil {
						c.GapCount++
						status("EVIDENCE_BUFFER_GAP")
					} else {
						status("REPORT_BUFFERED")
					}
				} else {
					lastProbe = c.ProbeNonce
					c.ProbeNonce = ack.ProbeNonce
					c.GapCount = 0
					status("REPORT_ACKNOWLEDGED")
				}
				if configuration != "" {
					lastConfig = configuration
				}
				if e = Save(path, c); e != nil {
					return e
				}
			}
			initial = false
		}
		if !now.Before(nextHeartbeat) {
			r, e := report(path, &c, "HEARTBEAT", nil)
			if e != nil {
				return e
			}
			ack, e := send(ctx, &c, r)
			if e == nil {
				c.ProbeNonce = ack.ProbeNonce
				if e = Save(path, c); e != nil {
					return e
				}
				_ = flush(ctx, &c, outbox)
			} else {
				var rejection *Rejected
				if errors.As(e, &rejection) && (rejection.Status == 401 || rejection.Status == 403) {
					return e
				}
				status("CONNECTION_UNAVAILABLE")
			}
			nextHeartbeat = now.Add(deviceprotocol.HeartbeatInterval)
		}
		select {
		case <-ctx.Done():
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			r, e := report(path, &c, "STOPPED", nil)
			if e == nil {
				if _, e = send(stopCtx, &c, r); e != nil {
					_ = enqueue(outbox, r)
				}
			}
			cancel()
			return nil
		case <-ticker.C:
		}
	}
}
