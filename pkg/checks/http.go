package checks

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	ct "github.com/sihuangtech/probelune-contracts"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync"
	"time"
)

func httpCheck(ctx context.Context, c ct.Check, p ct.Policy, r *ct.Result) error {
	var mu sync.Mutex
	timings := map[string]any{"dns_ms": nil, "connect_ms": nil, "tls_ms": nil, "ttfb_ms": nil}
	start := time.Now()
	var tlsStart time.Time
	trace := &httptrace.ClientTrace{
		TLSHandshakeStart: func() { mu.Lock(); tlsStart = time.Now(); mu.Unlock() },
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			mu.Lock()
			defer mu.Unlock()
			timings["tls_ms"] = float64(time.Since(tlsStart).Microseconds()) / 1000
			if err != nil {
				r.Stage = "tls"
			}
			certificate(c, state, r)
		},
		GotFirstResponseByte: func() {
			mu.Lock()
			defer mu.Unlock()
			timings["ttfb_ms"] = float64(time.Since(start).Microseconds()) / 1000
		},
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: c.InsecureTLS}, DisableKeepAlives: true, ResponseHeaderTimeout: time.Duration(c.Timeout) * time.Second, ForceAttemptHTTP2: true}
	defer transport.CloseIdleConnections()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		at := time.Now()
		ips, e := p.Resolve(ctx, host, c.Family)
		mu.Lock()
		timings["dns_ms"] = float64(time.Since(at).Microseconds()) / 1000
		mu.Unlock()
		if e != nil {
			mu.Lock()
			r.Stage = "dns_or_policy"
			if strings.Contains(e.Error(), "策略") {
				r.Status = "error"
			}
			mu.Unlock()
			return nil, e
		}
		at = time.Now()
		var conn net.Conn
		for _, ip := range ips {
			conn, e = (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if e == nil {
				break
			}
		}
		mu.Lock()
		timings["connect_ms"] = float64(time.Since(at).Microseconds()) / 1000
		mu.Unlock()
		return conn, e
	}
	client := http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if !c.FollowRedirects {
			return http.ErrUseLastResponse
		}
		if len(via) >= 5 {
			return fmt.Errorf("重定向超过 5 次")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("不允许的重定向协议")
		}
		if via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
			return fmt.Errorf("拒绝 HTTPS 到 HTTP 的降级重定向")
		}
		if _, e := p.Resolve(req.Context(), req.URL.Hostname(), c.Family); e != nil {
			r.Status = "error"
			r.Stage = "policy"
			return e
		}
		// 跨主机重定向移除所有自定义头，避免认证凭据泄漏。
		if req.URL.Host != via[0].URL.Host {
			req.Header = make(http.Header)
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), c.Method, c.Target, nil)
	if err != nil {
		return err
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "Probelune/"+ct.Version)
	res, err := client.Do(req)
	mu.Lock()
	r.Data["timings"] = timings
	mu.Unlock()
	if err != nil {
		return fmt.Errorf("HTTP 检查失败（%s）：%s", r.Stage, ct.Redact(err.Error()))
	}
	defer res.Body.Close()
	r.Stage = "http"
	r.Data["status_code"] = res.StatusCode
	r.Data["final_url"] = res.Request.URL.Scheme + "://" + res.Request.URL.Host + res.Request.URL.EscapedPath()
	if c.CertificateMinDays > 0 && res.TLS == nil {
		r.Status = "error"
		r.Stage = "configuration"
		return fmt.Errorf("证书有效期断言要求最终响应使用 HTTPS")
	}
	if res.TLS != nil {
		finalCheck := c
		finalCheck.Target = res.Request.URL.String()
		certificate(finalCheck, *res.TLS, r)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil {
		return err
	}
	r.Data["truncated"] = len(body) > 1<<20
	if len(body) > 1<<20 {
		body = body[:1<<20]
	}
	if c.ExpectedStatus > 0 && res.StatusCode != c.ExpectedStatus || c.ExpectedStatus == 0 && (res.StatusCode < 200 || res.StatusCode >= 400) {
		return fmt.Errorf("HTTP 状态码 %d 不符合预期", res.StatusCode)
	}
	if c.Contains != "" && !strings.Contains(string(body), c.Contains) {
		r.Stage = "assertion"
		return fmt.Errorf("响应不包含期望文本")
	}
	if c.CertificateMinDays > 0 && res.TLS != nil && len(res.TLS.PeerCertificates) > 0 && time.Until(res.TLS.PeerCertificates[0].NotAfter) < time.Duration(c.CertificateMinDays)*24*time.Hour {
		r.Stage = "certificate_expiry"
		return fmt.Errorf("证书剩余有效期不足 %d 天", c.CertificateMinDays)
	}
	r.Stage = "complete"
	return nil
}
func certificate(c ct.Check, state tls.ConnectionState, r *ct.Result) {
	if len(state.PeerCertificates) == 0 {
		return
	}
	cert := state.PeerCertificates[0]
	target, _ := url.Parse(c.Target)
	intermediates := x509.NewCertPool()
	for _, item := range state.PeerCertificates[1:] {
		intermediates.AddCert(item)
	}
	_, err := cert.Verify(x509.VerifyOptions{DNSName: target.Hostname(), Intermediates: intermediates})
	summary := map[string]any{"not_before": cert.NotBefore.Unix(), "not_after": cert.NotAfter.Unix(), "days_remaining": int(time.Until(cert.NotAfter).Hours() / 24), "verified": err == nil, "validation_bypassed": c.InsecureTLS}
	if err != nil {
		summary["error"] = ct.Redact(err.Error())
	}
	r.Data["certificate"] = summary
}
