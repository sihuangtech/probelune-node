# 巡汐 Probelune 探测节点

[English](README.md) · [网页面板](https://github.com/sihuangtech/probelune-dashboard) · [产品文档](https://github.com/sihuangtech/probelune)

在不同地区、运营商或内网部署轻量节点，主动连接自己的面板，执行结构化检测并上报结果。无需给节点开放入站端口。

持续监测支持 HTTP/HTTPS、TCP、ICMP、DNS；即时诊断额外支持系统已安装的 MTR/traceroute。默认验证 TLS；HTTP 支持状态码、正文关键字与重定向规则；ICMP 记录丢包和 RTT；DNS 使用独立解析器。访问私网需管理员明确配置 CIDR，不允许下发任意 shell。

## 编译与注册

安装 [Go](https://go.dev/doc/install)，当前验证的补丁版本为 **1.27.3**。

```sh
git clone https://github.com/sihuangtech/probelune-node.git
cd probelune-node
./scripts/build.sh
export PROBELUNE_REGISTER_TOKEN='面板生成的一次性令牌'
./dist/probelune-node --panel https://monitor.example.com --node-id 节点ID --data ./node-data
```

先在网页“探测节点”创建节点，注册令牌 10 分钟有效、只能使用一次。首次注册后身份保存在权限为 0600 的 `node-data/credential.json`。停止后清除环境变量，随后使用同一数据目录启动。已有身份会忽略残留注册变量，重新签发身份时显式加 `--register`。每个节点、每个面板实例使用独立数据目录。

本地受控 HTTP 联调需显式加 `--allow-http`；正式部署使用 HTTPS。默认并发 4，可通过 `--concurrency 1..32` 调整。`--version` 显示版本。

## Docker

```sh
docker build --pull -t probelune-node:local .
docker run --name probelune-node --restart unless-stopped \
  --cap-drop ALL --cap-add NET_RAW \
  -e PROBELUNE_PANEL=https://monitor.example.com \
  -e PROBELUNE_NODE_ID=节点ID \
  -e PROBELUNE_REGISTER_TOKEN=一次性令牌 \
  -v probelune-node-data:/data probelune-node:local
```

镜像使用 Go **1.27.3** 编译、Alpine **3.24.3** 运行，以 UID 65532 执行，包含 MTR/traceroute，仅授予需要的原始网络能力。Linux systemd 常驻运行及镜像刷新步骤见 [部署文档](deploy/README.md)。不同系统的 ICMP 权限和网络防火墙仍需实际核验。

## 断连与数据

任务租约 15 分钟，断连后仅继续执行尚未过期的缓存任务。离线结果缓冲上限为 100 MiB / 24 小时，淘汰数量会向面板报告。重传去重，迟到结果只影响历史，不覆盖新状态。面板离线、执行器出错、目标失败是三种不同情况。

本地单次检测可用 `--check` 从 stdin 读取 `{check,policy}` JSON；读取结果 `status` 判断成败。参见英文 README 的完整示例与 [协议文档](https://github.com/sihuangtech/probelune-contracts/blob/main/docs/PROTOCOL.md)。

开发检查：`go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./cmd/node`。当前面向小规模自部署，异地网络与生产容量需在实际环境验收。许可证：[GPL-3.0-or-later](LICENSE)。

## 星标历史

[![星标历史](https://api.star-history.com/svg?repos=sihuangtech/probelune-node&type=Date)](https://star-history.com/#sihuangtech/probelune-node&Date)

## 联系开发者

- 开发者: [Snake Konginchrist](https://github.com/Snake-Konginchrist)
- 邮箱: [developer@skstudio.cn](mailto:developer@skstudio.cn)
- 问题反馈: [GitHub Issues](https://github.com/sihuangtech/probelune-node/issues)
