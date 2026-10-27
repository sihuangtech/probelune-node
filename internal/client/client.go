package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	Base, NodeID, Token string
	HTTP                *http.Client
}

const Version = "0.1.0"

func New(base, nodeID, token string, allowHTTP bool) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("面板地址无效")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && allowHTTP) {
		return nil, fmt.Errorf("需要 HTTPS；受控开发环境可显式使用 --allow-http")
	}
	h := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("面板 API 不允许重定向") }}
	return &Client{Base: strings.TrimRight(base, "/"), NodeID: nodeID, Token: token, HTTP: h}, nil
}
func (c *Client) Post(ctx context.Context, path string, input, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.Base+"/api/v1/node/"+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-ID", c.NodeID)
	req.Header.Set("User-Agent", "Probelune-Node/0.1.0")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("面板连接失败：%w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		return ErrUnauthorized
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("面板返回 HTTP %d", res.StatusCode)
	}
	if output == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(output)
}

var ErrUnauthorized = fmt.Errorf("节点凭据失效，请重新注册")
