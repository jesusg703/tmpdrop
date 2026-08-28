package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testName = "test.txt"

func testLimits() Limits {
	return Limits{
		MaxFileSize:    1 << 20,
		MaxStorage:     1 << 30,
		DefaultTTL:     24 * time.Hour,
		QuotaDefault:   1 << 30,
		MaxFilesClient: 100,
	}
}

func newStore(t *testing.T, limits Limits) *Store {
	t.Helper()
	s, err := Open(t.TempDir(), limits)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func add(t *testing.T, s *Store, name string, content []byte, opt AddOptions) *File {
	t.Helper()
	f, err := s.Add(bytes.NewReader(content), name, "text/plain", opt)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	return f
}

func TestAddGetListDelete(t *testing.T) {
	s := newStore(t, testLimits())
	f := add(t, s, testName, []byte("hello"), AddOptions{ClientKey: "alice"})

	got, err := s.Get(f.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != testName || got.Size != 5 {
		t.Errorf("unexpected record: %+v", got)
	}
	if got.ExpiresAt.IsZero() {
		t.Errorf("expected default TTL to set an expiry")
	}

	files := s.List()
	if len(files) != 1 || files[0].ID != f.ID {
		t.Errorf("List = %+v", files)
	}

	if err := s.Delete(f.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(f.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after delete = %v", err)
	}
	if len(s.List()) != 0 {
		t.Errorf("List after delete not empty")
	}
}

func TestNameSanitized(t *testing.T) {
	s := newStore(t, testLimits())
	f := add(t, s, "../../../etc/passwd", []byte("x"), AddOptions{})
	if f.Name != "passwd" {
		t.Errorf("Name = %q, want passwd", f.Name)
	}
	f2 := add(t, s, "", []byte("y"), AddOptions{})
	if f2.Name == "" {
		t.Errorf("empty name should fall back to a default")
	}
}

func TestTooLarge(t *testing.T) {
	lim := testLimits()
	lim.MaxFileSize = 4
	s := newStore(t, lim)
	_, err := s.Add(bytes.NewReader([]byte("12345")), testName, "", AddOptions{})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if len(s.List()) != 0 {
		t.Errorf("oversize file must not be recorded")
	}
	entries, _ := os.ReadDir(filepath.Join(s.root, blobDirName))
	if len(entries) != 0 {
		t.Errorf("blob dir not clean, found %d entries", len(entries))
	}
}

func TestEmptyUpload(t *testing.T) {
	s := newStore(t, testLimits())
	_, err := s.Add(bytes.NewReader(nil), testName, "", AddOptions{})
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("err = %v, want ErrEmpty", err)
	}
}

func TestStorageFull(t *testing.T) {
	lim := testLimits()
	lim.MaxStorage = 8
	s := newStore(t, lim)
	add(t, s, "a.bin", []byte("12345"), AddOptions{ClientKey: "a"})
	_, err := s.Add(bytes.NewReader([]byte("12345")), "b.bin", "", AddOptions{ClientKey: "b"})
	if !errors.Is(err, ErrStorageFull) {
		t.Fatalf("err = %v, want ErrStorageFull", err)
	}
}

func TestClientQuota(t *testing.T) {
	lim := testLimits()
	lim.QuotaDefault = 8
	s := newStore(t, lim)
	add(t, s, "a.bin", []byte("12345"), AddOptions{ClientKey: "carol"})
	_, err := s.Add(bytes.NewReader([]byte("12345")), "b.bin", "", AddOptions{ClientKey: "carol"})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
	add(t, s, "c.bin", []byte("12345"), AddOptions{ClientKey: "dave"})
}

func TestClientFileLimit(t *testing.T) {
	lim := testLimits()
	lim.MaxFilesClient = 1
	s := newStore(t, lim)
	add(t, s, "a.bin", []byte("1"), AddOptions{ClientKey: "erin"})
	_, err := s.Add(bytes.NewReader([]byte("2")), "b.bin", "", AddOptions{ClientKey: "erin"})
	if !errors.Is(err, ErrFileLimit) {
		t.Fatalf("err = %v, want ErrFileLimit", err)
	}
}

func TestMaxDownloads(t *testing.T) {
	s := newStore(t, testLimits())
	f := add(t, s, testName, []byte("x"), AddOptions{MaxDownloads: 1})

	_, blob, err := s.OpenBlob(f.ID)
	if err != nil {
		t.Fatalf("first OpenBlob: %v", err)
	}
	blob.Close()

	exhausted, err := s.RecordDownload(f.ID)
	if err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}
	if !exhausted {
		t.Fatal("exhausted = false, want true after the last allowed download")
	}

	if _, _, err := s.OpenBlob(f.ID); !errors.Is(err, ErrDownloadsUsed) {
		t.Fatalf("second OpenBlob = %v, want ErrDownloadsUsed", err)
	}
}

func TestOpenBlobDoesNotCount(t *testing.T) {
	s := newStore(t, testLimits())
	f := add(t, s, testName, []byte("stream me"), AddOptions{})

	_, blob, err := s.OpenBlob(f.ID)
	if err != nil {
		t.Fatalf("OpenBlob: %v", err)
	}
	defer blob.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(blob)
	if buf.String() != "stream me" {
		t.Errorf("content = %q", buf.String())
	}

	got, err := s.Get(f.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Downloads != 0 {
		t.Errorf("Downloads = %d, want 0: opening must not count", got.Downloads)
	}
}

func TestAdoptStoresTempFile(t *testing.T) {
	s := newStore(t, testLimits())
	tmp, err := s.TempFile()
	if err != nil {
		t.Fatalf("TempFile: %v", err)
	}
	if _, err := tmp.WriteString("spooled"); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	f, err := s.Adopt(tmp.Name(), "spooled.txt", "text/plain", AddOptions{})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if f.Size != int64(len("spooled")) {
		t.Errorf("Size = %d, want %d", f.Size, len("spooled"))
	}
	if _, err := os.Stat(tmp.Name()); !errors.Is(err, os.ErrNotExist) {
		t.Error("temp file survived Adopt")
	}

	_, blob, err := s.OpenBlob(f.ID)
	if err != nil {
		t.Fatalf("OpenBlob: %v", err)
	}
	defer blob.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(blob)
	if buf.String() != "spooled" {
		t.Errorf("content = %q", buf.String())
	}
}

func TestExpiredFileAndSweep(t *testing.T) {
	dir := t.TempDir()
	lim := testLimits()
	s, err := Open(dir, lim)
	if err != nil {
		t.Fatal(err)
	}
	f := add(t, s, testName, []byte("data"), AddOptions{TTL: -1})

	manifestPath := filepath.Join(dir, manifestName)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Files map[string]*File `json:"files"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc.Files[f.ID].ExpiresAt = time.Now().Add(-time.Minute)
	out, _ := json.MarshalIndent(doc, "", "  ")
	if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
		t.Fatal(err)
	}

	s, err = Open(dir, lim)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(f.ID); !errors.Is(err, ErrExpired) {
		t.Fatalf("Get = %v, want ErrExpired", err)
	}
	if _, _, err := s.OpenBlob(f.ID); !errors.Is(err, ErrExpired) {
		t.Fatalf("OpenBlob = %v, want ErrExpired", err)
	}

	if removed := s.Sweep(time.Now()); removed != 1 {
		t.Fatalf("Sweep removed %d, want 1", removed)
	}
	if len(s.List()) != 0 {
		t.Errorf("list not empty after sweep")
	}
	s, err = Open(dir, lim)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.List()) != 0 {
		t.Errorf("manifest not persisted after sweep")
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	lim := testLimits()
	s, err := Open(dir, lim)
	if err != nil {
		t.Fatal(err)
	}
	f := add(t, s, testName, []byte("persist me"), AddOptions{Note: "n", TTL: 48 * time.Hour})

	s2, err := Open(dir, lim)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Get(f.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.Note != "n" || got.Size != int64(len("persist me")) {
		t.Errorf("record after reopen = %+v", got)
	}
	_, rc, err := s2.OpenBlob(f.ID)
	if err != nil {
		t.Fatalf("OpenBlob after reopen: %v", err)
	}
	rc.Close()
}

func TestCorruptManifestIgnored(t *testing.T) {
	dir := t.TempDir()
	lim := testLimits()
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir, lim); err == nil {
		t.Fatalf("expected error for corrupt manifest")
	}
}

func TestPasswordProtection(t *testing.T) {
	s := newStore(t, testLimits())
	f := add(t, s, "secret.txt", []byte("hidden"), AddOptions{Password: "hunter2"})

	if f.PasswordHash == "" {
		t.Fatalf("expected a password hash to be stored")
	}
	if f.PasswordHash == "hunter2" {
		t.Fatalf("plaintext password must not be stored")
	}

	required, ok, err := s.VerifyPassword(f.ID, "hunter2")
	if err != nil || !required || !ok {
		t.Fatalf("VerifyPassword(correct) = (%v,%v,%v), want (true,true,nil)", required, ok, err)
	}
	required, ok, err = s.VerifyPassword(f.ID, "wrong")
	if err != nil || !required || ok {
		t.Fatalf("VerifyPassword(wrong) = (%v,%v,%v), want (true,false,nil)", required, ok, err)
	}
	f2 := add(t, s, "secret2.txt", []byte("hidden"), AddOptions{Password: "hunter2"})
	if f.PasswordHash == f2.PasswordHash {
		t.Errorf("hashes should differ due to random salts")
	}
}

func TestPasswordOptional(t *testing.T) {
	s := newStore(t, testLimits())
	f := add(t, s, "open.txt", []byte("x"), AddOptions{})
	required, ok, err := s.VerifyPassword(f.ID, "")
	if err != nil || required || !ok {
		t.Fatalf("VerifyPassword on open file = (%v,%v,%v), want (false,true,nil)", required, ok, err)
	}
}

func TestDerivedFiles(t *testing.T) {
	s := newStore(t, testLimits())
	src := add(t, s, "a.txt", []byte("x"), AddOptions{})
	add(t, s, "a.pdf", []byte("p"), AddOptions{DerivedFrom: src.ID})
	add(t, s, "a.png", []byte("i"), AddOptions{DerivedFrom: src.ID})

	derived := s.Derived(src.ID)
	if len(derived) != 2 {
		t.Fatalf("Derived = %d files, want 2", len(derived))
	}
	if derived[0].Name != "a.pdf" || derived[1].Name != "a.png" {
		t.Errorf("Derived order = %v, %v", derived[0].Name, derived[1].Name)
	}
	if got := s.Derived(derived[0].ID); len(got) != 0 {
		t.Errorf("Derived on a leaf = %v, want none", got)
	}
}

func TestSweepDoesNotEatConcurrentUploads(t *testing.T) {
	s := newStore(t, testLimits())

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				s.Sweep(time.Now())
			}
		}
	}()

	for i := 0; i < 30; i++ {
		f, err := s.Add(bytes.NewReader([]byte("payload")), testName, "text/plain", AddOptions{})
		if err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
		if _, blob, err := s.OpenBlob(f.ID); err != nil {
			t.Fatalf("blob %d vanished: %v", i, err)
		} else {
			blob.Close()
		}
	}
	close(stop)
	<-done
}
