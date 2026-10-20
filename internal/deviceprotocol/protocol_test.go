package deviceprotocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestSignedMetadataRejectsTamperingWrongKeyAndKind(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	r := Report{Protocol: 1, DeviceID: "00000000-0000-4000-8000-000000000001", ID: "00000000-0000-4000-8000-000000000002", Epoch: 1, Sequence: 1, Kind: "HEARTBEAT", SentAt: time.Now().UTC(), AgentVersion: "1.0.0"}
	r.Sign(key)
	if r.Verify(pub) != nil || r.Verify(other) == nil {
		t.Fatal("device identity signature failure")
	}
	r.Sequence++
	if r.Verify(pub) == nil {
		t.Fatal("modified sequence signature accepted")
	}
	r.Kind = "SNAPSHOT"
	r.Sign(key)
	if r.Verify(pub) == nil {
		t.Fatal("snapshot kind without hardware accepted")
	}
	e := Enrollment{Code: "fixture-only", Name: "server", AgentVersion: "1.0.0"}
	e.Sign(key)
	if _, err := e.Verify(); err != nil {
		t.Fatal(err)
	}
	e.Name = "other-server"
	if _, err := e.Verify(); err == nil {
		t.Fatal("enrollment substitution accepted")
	}
}
