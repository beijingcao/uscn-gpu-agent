package gpuagent

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/beijingcao/uscn-gpu-agent/internal/deviceprotocol"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const Version = "1.0.1"

type Config struct {
	Platform       string `json:"platform"`
	PrivateKey     string `json:"private_key"`
	DeviceID       string `json:"device_id"`
	NodeID         string `json:"node_id"`
	Epoch          int64  `json:"epoch"`
	Sequence       int64  `json:"sequence"`
	NextReportUnix int64  `json:"next_report_unix"`
	ProbeNonce     string `json:"probe_nonce"`
	GapCount       int64  `json:"gap_count"`
}

func validatePlatform(raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Hostname() == "" || u.Path != "" && u.Path != "/" {
		return errors.New("invalid platform URL")
	}
	if u.Scheme == "https" {
		return nil
	}
	ip := net.ParseIP(u.Hostname())
	// Explicit trusted-LAN configuration also supports private and CGNAT mesh IPs.
	// Public HTTP endpoints remain rejected; the default endpoint uses HTTPS.
	privateHTTP := ip != nil && (ip.IsLoopback() || ip.IsPrivate())
	if v := ip.To4(); v != nil && v[0] == 100 && v[1] >= 64 && v[1] <= 127 {
		privateHTTP = true
	}
	if u.Scheme == "http" && privateHTTP {
		return nil
	}
	return errors.New("HTTPS platform required")
}
func NewConfig(platform string) (Config, error) {
	if e := validatePlatform(platform); e != nil {
		return Config{}, e
	}
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		return Config{}, e
	}
	return Config{Platform: strings.TrimSuffix(platform, "/"), PrivateKey: base64.StdEncoding.EncodeToString(key.Seed())}, nil
}
func (c Config) Key() (ed25519.PrivateKey, error) {
	seed, e := base64.StdEncoding.DecodeString(c.PrivateKey)
	if e != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("invalid protected device key")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
func Load(path string) (Config, error) {
	var c Config
	st, e := os.Lstat(path)
	if e != nil || !st.Mode().IsRegular() {
		return c, errors.New("ordinary device config required")
	}
	if !privateMode(st) {
		return c, errors.New("device config must be private")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return c, errors.New("device config unavailable")
	}
	if len(b) > 16384 {
		return c, errors.New("invalid device config")
	}
	if json.Unmarshal(b, &c) != nil {
		return c, errors.New("invalid device config")
	}
	if e = validatePlatform(c.Platform); e != nil {
		return c, e
	}
	if _, e = c.Key(); e != nil {
		return c, e
	}
	if c.DeviceID != "" && (!deviceprotocol.UUID.MatchString(c.DeviceID) || !deviceprotocol.UUID.MatchString(c.NodeID) || c.Epoch < 1) {
		return c, errors.New("invalid device binding")
	}
	return c, nil
}
func Save(path string, c Config) error {
	// State and signing keys never belong inside a source checkout.
	dir := filepath.Dir(path)
	abs, e := filepath.Abs(dir)
	if e != nil {
		return e
	}
	for p := abs; ; p = filepath.Dir(p) {
		if _, e := os.Stat(filepath.Join(p, "go.mod")); e == nil {
			return errors.New("device keys must remain outside source checkouts")
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	st, e := os.Lstat(dir)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("private state directory required")
	}
	if st, e := os.Lstat(path); e == nil && (!st.Mode().IsRegular() || !privateMode(st)) {
		return errors.New("protected ordinary config required")
	}
	raw, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(dir, ".gpu-config-")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0600); e == nil {
		_, e = tmp.Write(append(raw, '\n'))
	}
	if e == nil {
		e = tmp.Sync()
	}
	closeErr := tmp.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceConfig(name, path)
}
