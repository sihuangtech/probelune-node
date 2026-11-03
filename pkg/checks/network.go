package checks

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	probing "github.com/prometheus-community/pro-bing"
	ct "github.com/sihuangtech/probelune-contracts"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

func tcpCheck(ctx context.Context, c ct.Check, p ct.Policy, r *ct.Result) error {
	_, closeSocket, err := dialSocket(ctx, c, p, r, "tcp")
	if err != nil {
		return err
	}
	defer closeSocket()

	r.Stage = "complete"
	return nil
}
func pingCheck(ctx context.Context, c ct.Check, p ct.Policy, r *ct.Result) error {
	ips, err := p.Resolve(ctx, c.Target, c.Family)
	if err != nil {
		r.Status = "error"
		r.Stage = "policy"
		return err
	}
	ping, err := probing.NewPinger(ips[0].String())
	if err != nil {
		r.Status = "error"
		return err
	}
	ping.Count = c.Count
	ping.Size = c.Size
	ping.Timeout = time.Duration(c.Timeout) * time.Second
	ping.Interval = 500 * time.Millisecond
	ping.SetPrivileged(runtime.GOOS == "windows" || os.Getenv("PROBELUNE_ICMP_PRIVILEGED") == "1")
	if err = ping.RunWithContext(ctx); err != nil {
		r.Status = "error"
		r.Stage = "executor"
		return fmt.Errorf("ICMP 执行失败，请检查权限：%w", err)
	}
	stats := ping.Statistics()
	r.Data = map[string]any{"address": ips[0].String(), "sent": stats.PacketsSent, "received": stats.PacketsRecv, "loss_percent": stats.PacketLoss, "min_ms": float64(stats.MinRtt.Microseconds()) / 1000, "avg_ms": float64(stats.AvgRtt.Microseconds()) / 1000, "max_ms": float64(stats.MaxRtt.Microseconds()) / 1000}
	if stats.PacketLoss > c.MaxLoss {
		r.Stage = "icmp"
		return fmt.Errorf("丢包率 %.1f%% 超过阈值", stats.PacketLoss)
	}
	r.Stage = "complete"
	return nil
}

func udpCheck(ctx context.Context, c ct.Check, p ct.Policy, r *ct.Result) error {
	conn, closeSocket, err := dialSocket(ctx, c, p, r, "udp")
	if err != nil {
		return err
	}
	defer closeSocket()

	payload, _ := ct.UDPBytes(c.UDPPayload, c.UDPEncoding)
	expected, _ := ct.UDPBytes(c.UDPExpected, c.UDPEncoding)
	r.Stage = "send"
	if _, err = conn.Write(payload); err != nil {
		return err
	}
	r.Data["sent_bytes"] = len(payload)
	r.Stage = "response"
	response := make([]byte, 4097)
	n, err := conn.Read(response)
	if err != nil {
		r.Data["observation"] = "no_response"
		var timeout net.Error
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.As(err, &timeout) && timeout.Timeout() {
			r.Stage = "timeout"
			return fmt.Errorf("UDP timed out waiting for a response; this does not prove the port is closed")
		}
		if errors.Is(err, syscall.ECONNREFUSED) {
			r.Data["observation"] = "unreachable"
		}
		return err
	}
	r.Data["received_bytes"] = n
	r.Data["observation"] = "response"
	if n > 4096 {
		r.Data["truncated"] = true
		r.Stage = "assertion"
		return fmt.Errorf("UDP response exceeds the 4096-byte validation limit")
	}
	// Keep a bounded preview; request payloads are never copied into results.
	r.Data["response_hex"] = hex.EncodeToString(response[:min(n, 256)])
	if n == 0 || len(expected) > 0 && !bytes.Contains(response[:n], expected) {
		r.Stage = "assertion"
		return fmt.Errorf("UDP response did not match the expected content")
	}
	r.Stage = "complete"
	return nil
}

// dialSocket applies one policy-checked DNS resolution to both TCP and UDP.
// The returned cleanup also stops the cancellation callback.
func dialSocket(ctx context.Context, c ct.Check, p ct.Policy, r *ct.Result, network string) (net.Conn, func(), error) {
	r.Stage = "policy"
	ips, err := p.Resolve(ctx, c.Target, c.Family)
	if err != nil {
		r.Status = "error"
		return nil, nil, err
	}
	r.Stage = "connect"
	var conn net.Conn
	for _, ip := range ips {
		conn, err = (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), strconv.Itoa(c.Port)))
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err = conn.SetDeadline(deadline); err != nil {
			conn.Close()
			return nil, nil, err
		}
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	r.Data["remote_address"] = conn.RemoteAddr().String()
	return conn, func() { stop(); conn.Close() }, nil
}
