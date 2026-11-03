package checks

import (
	"context"
	ct "github.com/sihuangtech/probelune-contracts"
	"net"
	"strings"
	"testing"
	"time"
)

func udpFixture(t *testing.T, response []byte) int {
	t.Helper()
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			_, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			if response != nil {
				conn.WriteTo(response, addr)
			}
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func TestUDPResponseSemantics(t *testing.T) {
	policy := ct.Policy{AllowCIDRs: []string{"127.0.0.0/8"}}
	for _, tc := range []struct {
		name                    string
		response                []byte
		expected, status, stage string
	}{
		{"reply", []byte{0, 255, 1}, "ff", "success", "complete"},
		{"assertion", []byte{0, 255, 1}, "abcd", "failure", "assertion"},
		{"no reply", nil, "", "failure", "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := ct.Check{Type: "udp", Target: "127.0.0.1", Port: udpFixture(t, tc.response), Timeout: 1, UDPPayload: "1234", UDPEncoding: "hex", UDPExpected: tc.expected}
			r := Run(context.Background(), c, policy)
			if r.Status != tc.status || r.Stage != tc.stage {
				t.Fatalf("unexpected result: %+v", r)
			}
			if tc.response == nil && !strings.Contains(r.Error, "does not prove") {
				t.Fatal("timeout incorrectly described as closed port")
			}
		})
	}
}

func TestUDPPolicyAndCancellation(t *testing.T) {
	c := ct.Check{Type: "udp", Target: "127.0.0.1", Port: udpFixture(t, nil), Timeout: 10, UDPPayload: "ping"}
	if r := Run(context.Background(), c, ct.Policy{}); r.Status != "error" || r.Stage != "policy" {
		t.Fatalf("policy bypass: %+v", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(30*time.Millisecond, cancel)
	start := time.Now()
	r := Run(ctx, c, ct.Policy{AllowCIDRs: []string{"127.0.0.0/8"}})
	if r.Status != "cancelled" || time.Since(start) > time.Second {
		t.Fatalf("cancellation did not interrupt UDP read: %+v", r)
	}
}

func TestTCPUsesSharedNetworkPolicy(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	c := ct.Check{Type: ct.TypeTCP, Target: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Timeout: 1}
	if r := Run(context.Background(), c, ct.Policy{}); r.Status != "error" || r.Stage != "policy" {
		t.Fatalf("TCP policy bypass: %+v", r)
	}
	if r := Run(context.Background(), c, ct.Policy{AllowCIDRs: []string{"127.0.0.0/8"}}); r.Status != "success" || r.Stage != "complete" {
		t.Fatalf("TCP regression: %+v", r)
	}
}
