// Package checks 实现受控网络探测，可供节点和桌面诊断复用。
package checks

import (
	"context"
	"errors"
	ct "github.com/sihuangtech/probelune-contracts"
	"os/exec"
	"time"
)

func Capabilities() []string {
	out := []string{"http", "tcp", "dns", "ping", "certificate_expiry"}
	for _, kind := range []string{"mtr", "traceroute"} {
		if _, err := exec.LookPath(kind); err == nil {
			out = append(out, kind)
		}
	}
	return out
}
func Run(ctx context.Context, check ct.Check, policy ct.Policy) ct.Result {
	r := ct.Result{StartedAt: time.Now().Unix(), Status: "error", Stage: "validation", Data: map[string]any{}}
	if err := check.Validate(); err != nil {
		r.Error = err.Error()
		return r
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(check.Timeout)*time.Second)
	defer cancel()
	start := time.Now()
	first := ""
	for attempt := 0; attempt <= check.Retries; attempt++ {
		r.Status = "success"
		r.Error = ""
		r.Stage = "connect"
		r.Data = map[string]any{}
		r.Attempts = attempt + 1
		var err error
		switch check.Type {
		case "http":
			err = httpCheck(ctx, check, policy, &r)
		case "tcp":
			err = tcpCheck(ctx, check, policy, &r)
		case "dns":
			err = dnsCheck(ctx, check, policy, &r)
		case "ping":
			err = pingCheck(ctx, check, policy, &r)
		case "mtr", "traceroute":
			err = routeCheck(ctx, check, policy, &r)
		}
		if err == nil {
			break
		}
		r.Error = ct.Redact(err.Error())
		if first == "" {
			first = r.Error
		}
		if r.Status == "success" {
			r.Status = "failure"
		}
		if ctx.Err() != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				r.Status = "cancelled"
				r.Stage = "cancelled"
			} else {
				r.Stage = "timeout"
			}
			break
		}
	}
	ms := float64(time.Since(start).Microseconds()) / 1000
	r.Latency = &ms
	if r.Attempts > 1 {
		r.FirstError = first
	}
	return r
}
