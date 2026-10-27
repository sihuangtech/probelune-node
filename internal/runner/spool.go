package runner

import (
	"encoding/json"
	ct "github.com/sihuangtech/probelune-contracts"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func writeJSON(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.WriteFile(path+".tmp", raw, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func (r *Runner) saveResult(result ct.Result) error {
	return writeJSON(filepath.Join(r.Dir, "spool", result.ID+".json"), result)
}
func (r *Runner) batch() ([]ct.Result, error) {
	entries, err := os.ReadDir(filepath.Join(r.Dir, "spool"))
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		a, _ := entries[i].Info()
		b, _ := entries[j].Info()
		return a.ModTime().After(b.ModTime())
	})
	out := []ct.Result{}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		total += info.Size()
		path := filepath.Join(r.Dir, "spool", entry.Name())
		if total > 100<<20 || time.Since(info.ModTime()) > 24*time.Hour {
			os.Remove(path)
			r.mu.Lock()
			r.dropped++
			r.mu.Unlock()
			continue
		}
		if len(out) >= 50 {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var result ct.Result
		if json.Unmarshal(raw, &result) == nil {
			out = append(out, result)
		}
	}
	return out, nil
}
