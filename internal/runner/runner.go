package runner

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	ct "github.com/sihuangtech/probelune-contracts"
	"github.com/sihuangtech/probelune-node/internal/client"
	"github.com/sihuangtech/probelune-node/pkg/checks"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type running struct {
	cancel     context.CancelFunc
	assignment string
}
type Runner struct {
	Client  *client.Client
	Dir     string
	mu      sync.Mutex
	work    ct.Work
	active  map[string]running
	last    map[string]int64
	done    map[string]bool
	slots   chan struct{}
	dropped int
}

func New(c *client.Client, dir string, concurrency int) (*Runner, error) {
	if err := os.MkdirAll(filepath.Join(dir, "spool"), 0700); err != nil {
		return nil, err
	}
	r := &Runner{Client: c, Dir: dir, active: map[string]running{}, last: map[string]int64{}, done: map[string]bool{}, slots: make(chan struct{}, concurrency)}
	if raw, e := os.ReadFile(filepath.Join(dir, "work.json")); e == nil {
		json.Unmarshal(raw, &r.work)
	}
	if raw, e := os.ReadFile(filepath.Join(dir, "last.json")); e == nil {
		json.Unmarshal(raw, &r.last)
	}
	if raw, e := os.ReadFile(filepath.Join(dir, "done.json")); e == nil {
		json.Unmarshal(raw, &r.done)
	}
	return r, nil
}
func (r *Runner) Run(ctx context.Context) {
	go r.synchronize(ctx)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.mu.Lock()
			for _, v := range r.active {
				v.cancel()
			}
			r.mu.Unlock()
			return
		case <-ticker.C:
			r.schedule(ctx)
		}
	}
}
func (r *Runner) synchronize(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if err := r.sync(ctx); err != nil {
			slog.Warn("节点同步", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (r *Runner) sync(ctx context.Context) error {
	r.mu.Lock()
	info := ct.Poll{Protocol: ct.Protocol, Version: client.Version, Platform: runtime.GOOS + "/" + runtime.GOARCH, Capabilities: checks.Capabilities(), Queue: len(r.active), Dropped: r.dropped, Time: time.Now().Unix()}
	r.mu.Unlock()
	var work ct.Work
	if err := r.Client.Post(ctx, "poll", info, &work); err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			r.mu.Lock()
			r.work = ct.Work{}
			for _, v := range r.active {
				v.cancel()
			}
			writeJSON(filepath.Join(r.Dir, "work.json"), r.work)
			r.mu.Unlock()
		}
		return err
	}
	r.mu.Lock()
	r.work = work
	valid := map[string]bool{}
	for _, a := range work.Assignments {
		valid[a.ID] = true
	}
	for id := range r.last {
		if !valid[id] {
			delete(r.last, id)
		}
	}
	jobs := map[string]bool{}
	for _, j := range work.Jobs {
		jobs[j.ID] = true
	}
	for id := range r.done {
		if !jobs[id] {
			delete(r.done, id)
		}
	}
	for id, active := range r.active {
		if active.assignment != "" && !valid[active.assignment] {
			active.cancel()
		}
		for _, cancelID := range work.Cancelled {
			if cancelID == id {
				active.cancel()
			}
		}
	}
	err := writeJSON(filepath.Join(r.Dir, "work.json"), work)
	r.mu.Unlock()
	if err != nil {
		return err
	}
	batch, err := r.batch()
	if err != nil || len(batch) == 0 {
		return err
	}
	var answer struct {
		Accepted []string `json:"accepted"`
		Rejected []string `json:"rejected"`
	}
	if err = r.Client.Post(ctx, "results", batch, &answer); err != nil {
		return err
	}
	for _, id := range append(answer.Accepted, answer.Rejected...) {
		if validResultID(id) {
			os.Remove(filepath.Join(r.Dir, "spool", id+".json"))
		}
	}
	return nil
}
func (r *Runner) schedule(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().Unix()
	for _, a := range r.work.Assignments {
		if a.Until < now {
			continue
		}
		m := a.Monitor
		if m.Interval < 15 || m.Interval > 86400 || m.Check.Timeout >= m.Interval {
			continue
		}
		scheduled := now / int64(m.Interval) * int64(m.Interval)
		if scheduled < a.Start || r.last[a.ID] >= scheduled {
			continue
		}
		busy := false
		for _, active := range r.active {
			if active.assignment == a.ID {
				busy = true
			}
		}
		if busy {
			continue
		}
		result := ct.Result{ID: ct.RunID(m.ID, r.Client.NodeID, m.Version, scheduled), AssignmentID: a.ID, MonitorID: m.ID, NodeID: r.Client.NodeID, Version: m.Version, ScheduledAt: scheduled}
		if r.start(ctx, result, m.Check, a.Policy) {
			r.last[a.ID] = scheduled
			writeJSON(filepath.Join(r.Dir, "last.json"), r.last)
		}
	}
	for _, j := range r.work.Jobs {
		if r.done[j.ID] || j.Deadline < now {
			continue
		}
		if _, exists := r.active[j.ID]; exists {
			continue
		}
		result := ct.Result{ID: j.ID, JobID: j.ID, NodeID: r.Client.NodeID, ScheduledAt: now}
		r.start(ctx, result, j.Check, j.Policy)
	}
}
func (r *Runner) start(ctx context.Context, result ct.Result, check ct.Check, policy ct.Policy) bool {
	// 服务端下发的 ID 也必须校验，不能把远程字符串拼成任意文件路径。
	if !validResultID(result.ID) || policy.Validate() != nil {
		return false
	}
	select {
	case r.slots <- struct{}{}:
	default:
		return false
	}
	call, cancel := context.WithCancel(ctx)
	r.active[result.ID] = running{cancel: cancel, assignment: result.AssignmentID}
	go func() {
		persisted := false
		defer func() {
			cancel()
			<-r.slots
			r.mu.Lock()
			delete(r.active, result.ID)
			if result.JobID != "" && persisted {
				r.done[result.ID] = true
				writeJSON(filepath.Join(r.Dir, "done.json"), r.done)
			}
			r.mu.Unlock()
		}()
		measured := checks.Run(call, check, policy)
		measured.ID = result.ID
		measured.JobID = result.JobID
		measured.NodeID = result.NodeID
		measured.MonitorID = result.MonitorID
		measured.AssignmentID = result.AssignmentID
		measured.Version = result.Version
		measured.ScheduledAt = result.ScheduledAt
		if err := r.saveResult(measured); err != nil {
			slog.Error("结果缓冲失败", "error", err)
		} else {
			persisted = true
		}
	}()
	return true
}

func validResultID(id string) bool {
	if len(id) != 48 && len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
