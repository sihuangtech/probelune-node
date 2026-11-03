package runner

import (
	ct "github.com/sihuangtech/probelune-contracts"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type vanishedEntry string

func (e vanishedEntry) Name() string               { return string(e) }
func (e vanishedEntry) IsDir() bool                { return false }
func (e vanishedEntry) Type() fs.FileMode          { return 0 }
func (e vanishedEntry) Info() (fs.FileInfo, error) { return nil, os.ErrNotExist }

func TestSpoolIgnoresFilesRenamedDuringSnapshot(t *testing.T) {
	entries := []os.DirEntry{vanishedEntry("pending.json.tmp"), vanishedEntry("completed.json")}
	if files := spoolSnapshot(t.TempDir(), entries); len(files) != 0 {
		t.Fatal("vanished entries reached the sorted queue")
	}
}

func TestSpoolKeepsCompletedResultsAndExpiresOldOnes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "spool"), 0700); err != nil {
		t.Fatal(err)
	}
	r := &Runner{Dir: dir}
	if err := r.saveResult(ct.Result{ID: "fresh", Status: "success"}); err != nil {
		t.Fatal(err)
	}
	if err := r.saveResult(ct.Result{ID: "old", Status: "failure"}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "spool", "old.json"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spool", "unfinished.json.tmp"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	results, err := r.batch()
	if err != nil || len(results) != 1 || results[0].ID != "fresh" || r.dropped != 1 {
		t.Fatalf("invalid queue: %+v, dropped %d, error %v", results, r.dropped, err)
	}
}
