package checks

import (
	"bytes"
	"context"
	"fmt"
	ct "github.com/sihuangtech/probelune-contracts"
	"os"
	"os/exec"
	"strconv"
)

type cappedOutput struct{ bytes.Buffer }

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	left := 32768 - b.Len()
	if left > 0 {
		if left > n {
			left = n
		}
		b.Buffer.Write(p[:left])
	}
	return n, nil
}
func routeCheck(ctx context.Context, c ct.Check, p ct.Policy, r *ct.Result) error {
	ips, err := p.Resolve(ctx, c.Target, c.Family)
	if err != nil {
		r.Status = "error"
		r.Stage = "policy"
		return err
	}
	path, err := exec.LookPath(c.Type)
	if err != nil {
		r.Status = "error"
		r.Stage = "unsupported"
		return fmt.Errorf("节点未安装 %s", c.Type)
	}
	target := ips[0].String()
	args := []string{"-n", "-m", "20", "-q", "1", "-w", "1", target}
	if c.Type == "mtr" {
		args = []string{"--report", "--json", "--report-cycles", strconv.Itoa(c.Count), "--no-dns", target}
	}
	// 参数数组调用受控程序，目标先解析为 IP；不通过 shell 执行字符串。
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var output cappedOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	r.Data["output"] = output.String()
	r.Data["address"] = target
	r.Data["truncated"] = output.Len() >= 32768
	if err != nil {
		r.Status = "error"
		r.Stage = "executor"
		return fmt.Errorf("%s 执行失败：%w", c.Type, err)
	}
	r.Stage = "complete"
	return nil
}
