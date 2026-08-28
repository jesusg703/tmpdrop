package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSweepKeepsBlobsWhenManifestIsMissing(t *testing.T) {
	dir := t.TempDir()
	blobs := filepath.Join(dir, blobDirName)
	if err := os.MkdirAll(blobs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"aaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbb"} {
		if err := os.WriteFile(filepath.Join(blobs, id), []byte("user data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s, err := Open(dir, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	s.Sweep(time.Now())

	left, _ := os.ReadDir(blobs)
	if len(left) != 2 {
		t.Fatalf("blobs left = %d, want 2: a missing manifest must never destroy stored data", len(left))
	}
	if !s.ReclaimBlocked() {
		t.Error("ReclaimBlocked = false, want true so the operator is told the manifest is gone")
	}
}

func TestSweepDoesNotReclaimFreshBlobs(t *testing.T) {
	s := newStore(t, testLimits())
	add(t, s, testName, []byte("kept"), AddOptions{})

	stray := filepath.Join(s.blobs, "deadbeefdeadbeefdeadbeef")
	if err := os.WriteFile(stray, []byte("just written"), 0o644); err != nil {
		t.Fatal(err)
	}

	s.Sweep(time.Now())
	if _, err := os.Stat(stray); err != nil {
		t.Error("a blob written seconds ago was reclaimed; an upload in flight would be destroyed")
	}

	old := time.Now().Add(-2 * reclaimGrace)
	if err := os.Chtimes(stray, old, old); err != nil {
		t.Fatal(err)
	}
	s.Sweep(time.Now())
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Error("an old unreferenced blob should be reclaimed")
	}
}
