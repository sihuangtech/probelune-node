package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	ct "github.com/sihuangtech/probelune-contracts"
	"github.com/sihuangtech/probelune-node/internal/client"
	"github.com/sihuangtech/probelune-node/internal/runner"
	"github.com/sihuangtech/probelune-node/pkg/checks"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	panel := flag.String("panel", os.Getenv("PROBELUNE_PANEL"), "面板 HTTPS 地址")
	node := flag.String("node-id", os.Getenv("PROBELUNE_NODE_ID"), "面板分配的节点 ID")
	dir := flag.String("data", "./node-data", "私有状态目录")
	allowHTTP := flag.Bool("allow-http", false, "显式允许受控网络中的 HTTP")
	local := flag.Bool("check", false, "从 stdin 读取单次检测 JSON")
	concurrency := flag.Int("concurrency", 4, "最大并发检测数（1—32）")
	version := flag.Bool("version", false, "显示版本")
	register := flag.Bool("register", false, "使用新注册令牌替换已有身份")
	flag.Parse()
	if *version {
		fmt.Println(client.Version)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *local {
		var input struct {
			Check  ct.Check  `json:"check"`
			Policy ct.Policy `json:"policy"`
		}
		if err := json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&input); err != nil {
			die(err)
		}
		if err := input.Policy.Validate(); err != nil {
			die(err)
		}
		json.NewEncoder(os.Stdout).Encode(checks.Run(ctx, input.Check, input.Policy))
		return
	}
	if *concurrency < 1 || *concurrency > 32 {
		die(fmt.Errorf("并发数必须为 1—32"))
	}
	if err := os.MkdirAll(*dir, 0700); err != nil {
		die(err)
	}
	tokenPath := filepath.Join(*dir, "credential.json")
	var identity struct {
		Token  string `json:"token"`
		NodeID string `json:"node_id"`
		Panel  string `json:"panel"`
	}
	if raw, err := os.ReadFile(tokenPath); err == nil {
		if err = json.Unmarshal(raw, &identity); err != nil {
			die(err)
		}
	}
	if *node == "" {
		*node = identity.NodeID
	}
	c, err := client.New(*panel, *node, identity.Token, *allowHTTP)
	if err != nil {
		die(err)
	}
	// 长期身份绑定实例和节点，避免修改启动参数时向另一个面板泄露凭据。
	if identity.Token != "" && !*register && (identity.Panel != c.Base || identity.NodeID != *node) {
		die(fmt.Errorf("已保存身份与面板或节点不匹配，请使用新的数据目录，或设置注册令牌并显式 --register"))
	}
	if registration := os.Getenv("PROBELUNE_REGISTER_TOKEN"); registration != "" && (identity.Token == "" || *register) {
		c.Token = ""
		if err = c.Post(ctx, "register", map[string]string{"node_id": *node, "token": registration}, &identity); err != nil {
			die(err)
		}
		identity.Panel = c.Base
		raw, _ := json.Marshal(identity)
		if err = os.WriteFile(tokenPath, raw, 0600); err != nil {
			die(err)
		}
		c.Token = identity.Token
		os.Unsetenv("PROBELUNE_REGISTER_TOKEN")
	}
	if c.Token == "" || *node == "" {
		die(fmt.Errorf("请在面板创建节点并设置 PROBELUNE_REGISTER_TOKEN 完成首次注册"))
	}
	if identity.Panel != c.Base || identity.NodeID != *node {
		die(fmt.Errorf("注册未完成，拒绝使用不匹配的身份"))
	}
	worker, err := runner.New(c, *dir, *concurrency)
	if err != nil {
		die(err)
	}
	slog.Info("巡汐探测节点运行中", "version", client.Version)
	worker.Run(ctx)
}
func die(err error) { slog.Error("节点启动失败", "error", err); os.Exit(1) }
