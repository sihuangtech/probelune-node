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

type spoolFile struct {
	path     string
	size     int64
	modified time.Time
}

// Snapshot only completed files. Atomic writers may rename temporary entries
// between ReadDir and Info; a disappearing entry is not a queue failure.
func spoolSnapshot(dir string, entries []os.DirEntry) []spoolFile {
	files := []spoolFile{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, spoolFile{filepath.Join(dir, entry.Name()), info.Size(), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.After(files[j].modified) })
	return files
}

func (r *Runner) batch() ([]ct.Result, error) {
	entries, err := os.ReadDir(filepath.Join(r.Dir, "spool"))
	if err != nil {
		return nil, err
	}
	out := []ct.Result{}
	var total int64
	for _, file := range spoolSnapshot(filepath.Join(r.Dir, "spool"), entries) {
		total += file.size
		if total > 100<<20 || time.Since(file.modified) > 24*time.Hour {
			if os.Remove(file.path) != nil {
				continue
			}
			r.mu.Lock()
			r.dropped++
			r.mu.Unlock()
			continue
		}
		if len(out) >= 50 {
			continue
		}
		raw, err := os.ReadFile(file.path)
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
