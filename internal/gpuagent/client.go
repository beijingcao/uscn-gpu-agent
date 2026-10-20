package gpuagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/beijingcao/uscn-gpu-agent/internal/deviceprotocol"
	"io"
	"net/http"
	"os"
)

type Rejected struct {
	Code   string
	Status int
}

func (e *Rejected) Error() string { return "platform rejected device operation: " + e.Code }
func post(ctx context.Context, platform, path string, in, out any) error {
	raw, e := json.Marshal(in)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, platform+path, bytes.NewReader(raw))
	if e != nil {
		return errors.New("platform request unavailable")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "USCN-GPU-Agent/"+Version)
	client := http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(req)
	if e != nil {
		return errors.New("platform connection unavailable")
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, deviceprotocol.MaxBody+1))
	if e != nil || len(b) > deviceprotocol.MaxBody {
		return errors.New("invalid platform response")
	}
	if res.StatusCode != http.StatusOK {
		var v struct {
			Error json.RawMessage `json:"error"`
			Code  string          `json:"code"`
		}
		_ = json.Unmarshal(b, &v)
		code := v.Code
		var nested struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(v.Error, &nested) == nil && nested.Code != "" {
			code = nested.Code
		} else {
			_ = json.Unmarshal(v.Error, &code)
		}
		if len(code) > 128 || code == "" {
			code = "REQUEST_REJECTED"
		}
		return &Rejected{code, res.StatusCode}
	}
	if json.Unmarshal(b, out) != nil {
		return errors.New("invalid platform receipt")
	}
	return nil
}
func Enroll(ctx context.Context, path, platform, code, name string) (Config, error) {
	c, e := Load(path)
	if e != nil {
		if _, statErr := os.Lstat(path); statErr == nil {
			return c, e
		}
		c, e = NewConfig(platform)
		if e != nil {
			return c, e
		}
		if e = Save(path, c); e != nil {
			return c, e
		}
	}
	if c.DeviceID != "" {
		return c, nil
	}
	if c.Platform != platform {
		return c, errors.New("existing device belongs to another platform")
	}
	key, e := c.Key()
	if e != nil {
		return c, e
	}
	in := deviceprotocol.Enrollment{Code: code, Name: name, AgentVersion: Version}
	in.Sign(key)
	var receipt deviceprotocol.EnrollmentReceipt
	if e = post(ctx, c.Platform, "/api/v1/gpu-agent/enroll", in, &receipt); e != nil {
		return c, e
	}
	if !deviceprotocol.UUID.MatchString(receipt.DeviceID) || !deviceprotocol.UUID.MatchString(receipt.NodeID) || receipt.Epoch < 1 || receipt.ReportSeconds != 600 {
		return c, errors.New("invalid enrollment receipt")
	}
	c.DeviceID = receipt.DeviceID
	c.NodeID = receipt.NodeID
	c.Epoch = receipt.Epoch
	return c, Save(path, c)
}
