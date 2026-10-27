# Host deployment

The source build uses Go **1.27.3**. Its Docker builder is `golang:1.27.3-alpine3.24`, and the runtime is `alpine:3.24.3`, verified on 2026-10-28 against [Go downloads](https://go.dev/dl/) and the [Alpine image](https://hub.docker.com/_/alpine). Each repository configures a weekly Dependabot check for its Dockerfile.

Build with `./scripts/build.sh`. Cross-build with `GOOS=linux GOARCH=amd64 ./scripts/build.sh` (or `arm64`). For Windows add the `.exe` suffix to the output filename; MTR is optional and system-dependent.

For a Debian/Ubuntu systemd host:

```sh
sudo useradd --system --home /var/lib/probelune-node --shell /usr/sbin/nologin probelune-node
sudo install -m 755 dist/probelune-node /usr/local/bin/probelune-node
sudo install -d -m 700 /etc/probelune
sudo install -m 600 deploy/node.env.example /etc/probelune/node.env
sudo install -m 644 deploy/probelune-node.service /etc/systemd/system/probelune-node.service
```

Edit `/etc/probelune/node.env` locally with the dashboard URL, node ID and fresh registration token. Then:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now probelune-node
sudo journalctl -u probelune-node -n 30
```

Remove the one-time registration token from the environment file after registration. The systemd service receives `CAP_NET_RAW` only, uses a private state directory and cannot write the rest of the filesystem. Install your distribution's `mtr` and `traceroute` packages to enable route diagnostics. Depending on the distribution, `mtr-packet` may also need the raw-network capability. Do not run the whole desktop or node service as root to work around a missing tool.

On macOS, unprivileged ICMP normally works. Install route tools separately if needed. On Windows, raw ICMP may require elevation; HTTP, DNS and TCP remain independent. A missing or denied tool is an executor error and never proves the target is down.

Upgrade: stop service, preserve its state directory, replace the binary, start service, check reported version/capabilities. Never copy the same credentials to multiple machines. For rotation, stop the service, generate new credentials in the dashboard, and run registration once with `--register` under the service identity before restarting. For revocation, use the dashboard's revoke action and remove the service when no longer needed.

Troubleshooting: check outbound HTTPS reachability, system clock (within 30 seconds), CIDR policy, disk space, ownership and capabilities. `queue` counts active executions; `dropped` counts buffered results discarded by retention/capacity. Keep the credential file and spool private.

## Container updates

Use `docker build --pull --no-cache -t probelune-node:local .` when refreshing a deployment. `--pull` refreshes the specified base image tags; `--no-cache` reruns the package installation. `apk add --no-cache` obtains ca-certificates, tzdata, MTR, traceroute and libcap from the current indexes of the selected stable Alpine branch. The image versions are explicit stable patches; update them after checking upstream releases and rebuild before recreating the node container on its existing data volume.
