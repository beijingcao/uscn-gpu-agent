package gpuagent

import "testing"

func TestExplicitPlatformTransportBoundaries(t *testing.T) {
	for _, url := range []string{"https://uscn.si", "http://127.0.0.1:18443", "http://192.168.0.10:18443", "http://100.64.0.2:18443", "http://[fd00::1]:18443"} {
		if err := validatePlatform(url); err != nil {
			t.Errorf("trusted explicit endpoint rejected: %s", url)
		}
	}
	for _, url := range []string{"http://example.com", "http://8.8.8.8", "http://100.128.0.1", "https://user:secret@example.com", "https://example.com/?key=secret", "file:///tmp/device"} {
		if err := validatePlatform(url); err == nil {
			t.Errorf("unsafe endpoint accepted: %s", url)
		}
	}
}
