// Package deviceprotocol is the versioned, signed metadata-only GPU agent wire
// contract. Browser sessions and device signing keys are different identities.
package deviceprotocol

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/beijingcao/uscn-gpu-agent/internal/hardwareinventory"
	"regexp"
	"time"
)

const Version = 1
const ReportInterval = 600 * time.Second
const HeartbeatInterval = 15 * time.Second
const MaxBody = 256 << 10

var UUID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

type Enrollment struct {
	Code         string `json:"code"`
	PublicKey    string `json:"public_key"`
	Name         string `json:"name"`
	AgentVersion string `json:"agent_version"`
	Signature    string `json:"signature"`
}
type EnrollmentReceipt struct {
	DeviceID      string `json:"device_id"`
	NodeID        string `json:"node_id"`
	Epoch         int64  `json:"epoch"`
	ReportSeconds int    `json:"report_seconds"`
}
type Report struct {
	Protocol     int                         `json:"protocol"`
	DeviceID     string                      `json:"device_id"`
	Epoch        int64                       `json:"epoch"`
	ID           string                      `json:"id"`
	Sequence     int64                       `json:"sequence"`
	SentAt       time.Time                   `json:"sent_at"`
	Kind         string                      `json:"kind"`
	AgentVersion string                      `json:"agent_version"`
	ProbeNonce   string                      `json:"probe_nonce,omitempty"`
	GapCount     int64                       `json:"gap_count"`
	Snapshot     *hardwareinventory.Snapshot `json:"snapshot,omitempty"`
	Signature    string                      `json:"signature"`
}
type Ack struct {
	ID            string    `json:"id"`
	ReceivedAt    time.Time `json:"received_at"`
	Fresh         bool      `json:"fresh"`
	ProbeNonce    string    `json:"probe_nonce"`
	ReportSeconds int       `json:"report_seconds"`
}

func (v Enrollment) Payload() []byte {
	v.Signature = ""
	b, _ := json.Marshal(v)
	return append([]byte("USCN_GPU_ENROLL_V1\n"), b...)
}
func (v *Enrollment) Sign(key ed25519.PrivateKey) {
	v.PublicKey = base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	v.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, v.Payload()))
}
func (v Enrollment) Verify() (ed25519.PublicKey, error) {
	key, e := base64.StdEncoding.DecodeString(v.PublicKey)
	if e != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("invalid enrollment key")
	}
	sig, e := base64.StdEncoding.DecodeString(v.Signature)
	if e != nil || !ed25519.Verify(key, v.Payload(), sig) {
		return nil, errors.New("invalid enrollment signature")
	}
	return key, nil
}
func (v Report) Payload() []byte {
	v.Signature = ""
	b, _ := json.Marshal(v)
	return append([]byte("USCN_GPU_REPORT_V1\n"), b...)
}
func (v *Report) Sign(key ed25519.PrivateKey) {
	v.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, v.Payload()))
}
func (v Report) Verify(key ed25519.PublicKey) error {
	if v.Protocol != Version || !UUID.MatchString(v.DeviceID) || !UUID.MatchString(v.ID) || v.Epoch < 1 || v.Sequence < 1 || v.Sequence > 1<<52 || v.SentAt.IsZero() || v.GapCount < 0 || len(v.AgentVersion) > 64 || len(v.ProbeNonce) > 128 {
		return errors.New("invalid device report")
	}
	if v.Kind != "SNAPSHOT" && v.Kind != "HEARTBEAT" && v.Kind != "STOPPED" && v.Kind != "COLLECTION_ERROR" {
		return errors.New("invalid report kind")
	}
	if (v.Kind == "SNAPSHOT") != (v.Snapshot != nil) {
		return errors.New("invalid snapshot envelope")
	}
	if v.Snapshot != nil {
		if e := v.Snapshot.Validate(); e != nil {
			return e
		}
	}
	sig, e := base64.StdEncoding.DecodeString(v.Signature)
	if e != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, v.Payload(), sig) {
		return errors.New("invalid report signature")
	}
	return nil
}
