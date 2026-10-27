# Probelune Node

[简体中文](README.zh-CN.md) · [Dashboard](https://github.com/sihuangtech/probelune-dashboard) · [Product / PRD](https://github.com/sihuangtech/probelune)

Deployable probes for **Probelune (巡汐)**. Run a node on each network you want to observe; it connects outward to your dashboard, executes structured checks, and buffers results during brief disconnections. No inbound node port is required.

## Capabilities

| Continuous monitoring | One-off diagnostics |
| --- | --- |
| HTTP/HTTPS, TCP, ICMP, DNS | The same checks plus MTR and traceroute when installed |

HTTP verifies TLS by default, records connection timing, supports status/text assertions and bounded redirects. ICMP reports loss and RTT; DNS uses an independent resolver. Private targets require explicit CIDRs set by the dashboard administrator. Tasks are data, never arbitrary shell commands.

## Build and register

Requires Go 1.27+. No adjacent repository is required.

```sh
git clone https://github.com/sihuangtech/probelune-node.git
cd probelune-node
./scripts/build.sh
```

Create a node in the dashboard's **Nodes** page and use its one-time token within ten minutes:

```sh
export PROBELUNE_REGISTER_TOKEN='ONE_TIME_TOKEN'
./dist/probelune-node --panel https://monitor.example.com --node-id NODE_ID --data ./node-data
```

The node stores its identity under `node-data/credential.json` (0600). Stop with Ctrl-C; unset the registration variable before later runs. Restart with the same panel, node ID and data directory. An already registered node ignores a leftover registration variable; explicitly use `--register` when rotating credentials. Do not share data directories between nodes or dashboard instances.

For an isolated development network only, explicitly add `--allow-http`. Set `--concurrency` between 1 and 32 (default 4). `--version` prints the implementation version.

## Container

```sh
docker build -t probelune-node:local .
docker run --name probelune-node --restart unless-stopped \
  --cap-drop ALL --cap-add NET_RAW \
  -e PROBELUNE_PANEL=https://monitor.example.com \
  -e PROBELUNE_NODE_ID=NODE_ID \
  -e PROBELUNE_REGISTER_TOKEN=ONE_TIME_TOKEN \
  -v probelune-node-data:/data probelune-node:local
```

The image runs as UID 65532, includes MTR/traceroute and grants only raw-network capability to the relevant executables. It does not require privileged mode or host networking. ICMP results still depend on the host/kernel/network policy. For a host service, see [deployment instructions](deploy/README.md).

## Local diagnostic mode

```sh
printf '%s\n' '{"check":{"type":"http","target":"https://example.com","timeout":10},"policy":{"allow_cidrs":[]}}' | ./dist/probelune-node --check
```

The command emits one JSON result and never creates a continuous monitor. Explicitly add a CIDR if you intentionally want to inspect a private endpoint. A command success exit does not mean the target check succeeded; inspect the result's `status`.

## Reliability and limits

Assignments expire after 15 minutes. Cached tasks continue only until their lease ends. The file spool is capped at 100 MiB and 24 hours; discarded entries are counted. Uploads are idempotent, and late results remain historical evidence instead of replacing fresh state. Server disconnection, permission errors and target failures are different conditions.

This release targets small self-hosted networks. Production capacity, real geographic diversity and provider-specific behavior require deployment validation. See [protocol semantics](https://github.com/sihuangtech/probelune-contracts/blob/main/docs/PROTOCOL.md), [SECURITY.md](SECURITY.md) and [CHANGELOG.md](CHANGELOG.md).

Development: `go fmt ./...`, `go vet ./...`, `go test ./...`, `go build ./cmd/node`. Distributed under [GPL-3.0-or-later](LICENSE).
