package checks

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	ct "github.com/sihuangtech/probelune-contracts"
	"net"
	"sort"
	"strings"
)

func dnsCheck(ctx context.Context, c ct.Check, p ct.Policy, r *ct.Result) error {
	server := c.Resolver
	if server == "" {
		config, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil || len(config.Servers) == 0 {
			return systemDNS(ctx, c, r)
		}
		server = net.JoinHostPort(config.Servers[0], config.Port)
	} else {
		host, port, e := net.SplitHostPort(server)
		if e != nil {
			host = server
			port = "53"
		}
		ips, e := p.Resolve(ctx, host, c.Family)
		if e != nil {
			r.Status = "error"
			r.Stage = "policy"
			return e
		}
		server = net.JoinHostPort(ips[0].String(), port)
	}
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(c.Target), dns.StringToType[c.DNSRecord])
	client := new(dns.Client)
	answer, _, err := client.ExchangeContext(ctx, msg, server)
	if err != nil {
		return err
	}
	if answer.Truncated {
		client.Net = "tcp"
		answer, _, err = client.ExchangeContext(ctx, msg, server)
		if err != nil {
			return err
		}
	}
	if answer.Rcode != dns.RcodeSuccess {
		r.Stage = "dns"
		return fmt.Errorf("DNS %s", dns.RcodeToString[answer.Rcode])
	}
	records := []string{}
	for _, record := range answer.Answer {
		records = append(records, record.String())
	}
	sort.Strings(records)
	r.Data["resolver"] = server
	r.Data["records"] = records
	if len(records) == 0 {
		return fmt.Errorf("DNS 没有返回所需记录")
	}
	if c.ExpectedDNS != "" && !strings.Contains(strings.Join(records, "\n"), c.ExpectedDNS) {
		r.Stage = "assertion"
		return fmt.Errorf("DNS 记录不符合预期")
	}
	r.Stage = "complete"
	return nil
}
func systemDNS(ctx context.Context, c ct.Check, r *ct.Result) error {
	if c.DNSRecord != "A" && c.DNSRecord != "AAAA" {
		r.Status = "error"
		return fmt.Errorf("此系统需要显式配置 DNS 解析器以查询 %s", c.DNSRecord)
	}
	family := "ip4"
	if c.DNSRecord == "AAAA" {
		family = "ip6"
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, family, c.Target)
	if err != nil {
		return err
	}
	records := []string{}
	for _, ip := range ips {
		records = append(records, ip.String())
	}
	r.Data["resolver"] = "system"
	r.Data["records"] = records
	if c.ExpectedDNS != "" && !strings.Contains(strings.Join(records, "\n"), c.ExpectedDNS) {
		return fmt.Errorf("DNS 记录不符合预期")
	}
	r.Stage = "complete"
	return nil
}
