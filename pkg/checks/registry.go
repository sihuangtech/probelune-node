package checks

import (
	"context"
	ct "github.com/sihuangtech/probelune-contracts"
	"os/exec"
)

type executor func(context.Context, ct.Check, ct.Policy, *ct.Result) error
type registration struct {
	kind string
	run  executor
	tool string
}

// One registration drives dispatch and advertised protocol capabilities.
var registry = []registration{
	{ct.TypeHTTP, httpCheck, ""},
	{ct.TypeTCP, tcpCheck, ""},
	{ct.TypeUDP, udpCheck, ""},
	{ct.TypePing, pingCheck, ""},
	{ct.TypeDNS, dnsCheck, ""},
	{ct.TypeMTR, routeCheck, "mtr"},
	{ct.TypeTraceroute, routeCheck, "traceroute"},
}

func executorFor(kind string) (executor, bool) {
	for _, entry := range registry {
		if entry.kind == kind {
			return entry.run, true
		}
	}
	return nil, false
}

func Capabilities() []string {
	out := []string{}
	for _, entry := range registry {
		if entry.tool != "" {
			if _, err := exec.LookPath(entry.tool); err != nil {
				continue
			}
		}
		out = append(out, entry.kind)
	}
	return append(out, ct.CapabilityCertificateExpiry)
}
