package checks

import (
	"context"
	"fmt"
	probing "github.com/prometheus-community/pro-bing"
	ct "github.com/sihuangtech/probelune-contracts"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"
)

func tcpCheck(ctx context.Context, c ct.Check, p ct.Policy, r *ct.Result) error {
	if _, err := p.Resolve(ctx, c.Target, c.Family); err != nil {
		r.Status = "error"
		r.Stage = "policy"
		return err
	}
	conn, err := p.Dial(c.Family)(ctx, "tcp", net.JoinHostPort(c.Target, strconv.Itoa(c.Port)))
	if err != nil {
		return err
	}
	defer conn.Close()
	r.Data["remote_address"] = conn.RemoteAddr().String()
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
