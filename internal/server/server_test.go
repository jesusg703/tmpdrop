package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesusg703/tmpdrop/internal/config"
	"github.com/jesusg703/tmpdrop/internal/store"
)

func testConfig() config.Config {
	return config.Config{
		Addr:            ":0",
		MaxFileSize:     1 << 20,
		MaxStorage:      1 << 30,
		DefaultTTL:      24 * time.Hour,
		SweepInterval:   time.Minute,
		ShutdownTimeout: 10 * time.Second,
		QuotaDefault:    1 << 30,
		MaxFilesClient:  100,
		LogLevel:        "info",
	}
}

func newTestServer(t *testing.T, dir string, cfg config.Config) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if cfg.StorageDir == "" {
		cfg.StorageDir = dir
	}
	st, err := store.Open(cfg.StorageDir, store.Limits{
		MaxFileSize:    cfg.MaxFileSize,
		MaxStorage:     cfg.MaxStorage,
		DefaultTTL:     cfg.DefaultTTL,
		QuotaDefault:   cfg.QuotaDefault,
		MaxFilesClient: cfg.MaxFilesClient,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	srv := httptest.NewServer(New(st, cfg, log).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func uploadFile(t *testing.T, base, name, content string, fields map[string]string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/upload", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func uploadOK(t *testing.T, base, name, content string, fields map[string]string) map[string]any {
	t.Helper()
	res := uploadFile(t, base, name, content, fields)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, want 201", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q", body)
	}
}

func TestIndexPage(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q", ct)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "tmpdrop") {
		t.Errorf("index does not mention tmpdrop")
	}
}

func TestUploadDownloadDelete(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())

	created := uploadOK(t, srv.URL, "hello.txt", "hello world", map[string]string{"note": "hi"})
	id := created["id"].(string)
	if created["name"] != "hello.txt" {
		t.Errorf("name = %v", created["name"])
	}

	res, err := http.Get(srv.URL + "/d/" + id)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "hello world" {
		t.Errorf("download = %d %q", res.StatusCode, body)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, "hello.txt") {
		t.Errorf("Content-Disposition = %q", cd)
	}

	res, err = http.Get(srv.URL + "/d/" + id + "?inline=1")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if cd := res.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Errorf("inline Content-Disposition = %q", cd)
	}

	res, err = http.Get(srv.URL + "/api/files")
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Files []fileDTO `json:"files"`
		Count int       `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if list.Count != 1 || list.Files[0].ID != id {
		t.Errorf("list = %+v", list)
	}
	if list.Files[0].ConvertAvailable {
		t.Errorf("ConvertAvailable should be false when convertx is disabled")
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/files/"+id, nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d", res.StatusCode)
	}

	res, err = http.Get(srv.URL + "/d/" + id)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status after delete = %d, want 404", res.StatusCode)
	}
}

func TestDownloadLimit(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer(t, dir, testConfig())
	created := uploadOK(t, srv.URL, "once.txt", "only once", map[string]string{"max_downloads": "1"})
	id := created["id"].(string)

	res, _ := http.Get(srv.URL + "/d/" + id)
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("first download = %d", res.StatusCode)
	}

	if _, err := os.Stat(filepath.Join(dir, "files", id)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("blob still on disk after the last download: %v", err)
	}

	res, _ = http.Get(srv.URL + "/d/" + id)
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("second download = %d, want 404", res.StatusCode)
	}
}

func TestHeadDoesNotCountDownload(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	created := uploadOK(t, srv.URL, "head.txt", "body bytes", map[string]string{"max_downloads": "1"})
	id := created["id"].(string)

	res, err := http.Head(srv.URL + "/d/" + id)
	if err != nil {
		t.Fatalf("HEAD: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("HEAD = %d, want 200", res.StatusCode)
	}

	if got := downloadsOf(t, srv.URL, id); got != 0 {
		t.Errorf("downloads after HEAD = %d, want 0", got)
	}

	res, _ = http.Get(srv.URL + "/d/" + id)
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("download after HEAD = %d, want 200", res.StatusCode)
	}
}

func TestRangeRequestServesPartialWithoutCounting(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	created := uploadOK(t, srv.URL, "ranged.txt", "0123456789", nil)
	id := created["id"].(string)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/d/"+id, nil)
	req.Header.Set("Range", "bytes=0-3")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ranged GET: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()

	if res.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", res.StatusCode)
	}
	if string(body) != "0123" {
		t.Errorf("body = %q, want %q", body, "0123")
	}
	if cr := res.Header.Get("Content-Range"); cr != "bytes 0-3/10" {
		t.Errorf("Content-Range = %q", cr)
	}
	if got := downloadsOf(t, srv.URL, id); got != 0 {
		t.Errorf("downloads after a ranged request = %d, want 0", got)
	}
}

func TestUploadedHTMLIsNeverInline(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	created := uploadOK(t, srv.URL, "page.html", "<html><body>hi</body></html>", nil)
	id := created["id"].(string)

	res, err := http.Get(srv.URL + "/d/" + id + "?inline=1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()

	if cd := res.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment for HTML", cd)
	}
	if csp := res.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
		t.Errorf("Content-Security-Policy = %q, want a sandbox on served files", csp)
	}
}

func downloadsOf(t *testing.T, base, id string) int {
	t.Helper()
	res, err := http.Get(base + "/api/files/" + id)
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	defer res.Body.Close()
	var dto struct {
		Downloads int `json:"downloads"`
	}
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	return dto.Downloads
}

func TestExpiredFileReturnsGone(t *testing.T) {
	dir := t.TempDir()
	id := strings.Repeat("ab", 12)

	os.MkdirAll(filepath.Join(dir, "files"), 0o755)
	if err := os.WriteFile(filepath.Join(dir, "files", id), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := &store.File{
		ID:         id,
		Name:       "old.txt",
		Size:       4,
		Mime:       "text/plain",
		UploadedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt:  time.Now().Add(-time.Hour),
	}
	doc := map[string]any{"version": 1, "files": map[string]*store.File{id: rec}}
	data, _ := json.MarshalIndent(doc, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer(t, dir, testConfig())
	res, err := http.Get(srv.URL + "/d/" + id)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusGone {
		t.Errorf("status = %d, want 410", res.StatusCode)
	}
}

func TestUploadTooLarge(t *testing.T) {
	cfg := testConfig()
	cfg.MaxFileSize = 4
	srv := newTestServer(t, t.TempDir(), cfg)

	res := uploadFile(t, srv.URL, "big.bin", "12345", nil)
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", res.StatusCode)
	}
}

func TestUploadQuotaExceeded(t *testing.T) {
	cfg := testConfig()
	cfg.QuotaDefault = 4
	srv := newTestServer(t, t.TempDir(), cfg)

	res := uploadFile(t, srv.URL, "big.bin", "12345", nil)
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", res.StatusCode)
	}
}

func TestGetFileMetadata(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	created := uploadOK(t, srv.URL, "meta.txt", "x", nil)
	id := created["id"].(string)

	res, err := http.Get(srv.URL + "/api/files/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var dto fileDTO
	if err := json.NewDecoder(res.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}
	if dto.Name != "meta.txt" || dto.URL != "/d/"+id+"?inline=1" {
		t.Errorf("dto = %+v", dto)
	}
	if dto.Protected {
		t.Errorf("file uploaded without password should not be protected")
	}
	if len(dto.Formats) != 1 || dto.Formats[0].Ext != "txt" {
		t.Errorf("Formats = %+v, want just the source format", dto.Formats)
	}
	if len(dto.Targets) == 0 {
		t.Errorf("Targets = %v, want convertible formats for a txt file", dto.Targets)
	}
}

func TestPasswordProtectedDownload(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	created := uploadOK(t, srv.URL, "locked.txt", "classified", map[string]string{"password": "swordfish"})
	id := created["id"].(string)
	if created["protected"] != true {
		t.Fatalf("protected = %v, want true", created["protected"])
	}

	res, _ := http.Get(srv.URL + "/d/" + id)
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("no password: status = %d, want 401", res.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/d/"+id, nil)
	req.Header.Set("X-File-Password", "nope")
	res, _ = http.DefaultClient.Do(req)
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("wrong password: status = %d, want 403", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/d/"+id, nil)
	req.Header.Set("X-File-Password", "swordfish")
	res, _ = http.DefaultClient.Do(req)
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "classified" {
		t.Errorf("correct password: status = %d body = %q", res.StatusCode, body)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/files/"+id, nil)
	res, _ = http.DefaultClient.Do(req)
	var meta fileDTO
	json.NewDecoder(res.Body).Decode(&meta)
	res.Body.Close()
	if meta.Downloads != 1 {
		t.Errorf("Downloads = %d, want 1 (only the successful delivery)", meta.Downloads)
	}
}

func TestConvertDisabledReturns503(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	created := uploadOK(t, srv.URL, "a.mp4", "x", nil)
	id := created["id"].(string)

	body := strings.NewReader(`{"target":"mp3"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/files/"+id+"/convert", body)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", res.StatusCode)
	}
}

func TestForwardedForNeedsATrustedProxy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		trusted []string
		remote  string
		fwd     string
		want    string
	}{
		{"untrusted caller is not believed", nil, "203.0.113.9:1234", "1.2.3.4", "203.0.113.9"},
		{"trusted proxy is believed", []string{"10.0.0.0/8"}, "10.1.2.3:5678", "1.2.3.4", "1.2.3.4"},
		{"first hop wins", []string{"10.0.0.0/8"}, "10.1.2.3:5678", "1.2.3.4, 10.9.9.9", "1.2.3.4"},
		{"garbage falls back", []string{"10.0.0.0/8"}, "10.1.2.3:5678", "not-an-ip", "10.1.2.3"},
		{"bare ip is a valid entry", []string{"10.1.2.3"}, "10.1.2.3:5678", "1.2.3.4", "1.2.3.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.TrustedProxies = tc.trusted
			s := New(nil, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			req.Header.Set("X-Forwarded-For", tc.fwd)

			if got := s.clientIP(req); got != tc.want {
				t.Errorf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPasswordAttemptsAreLimited(t *testing.T) {
	srv := newTestServer(t, t.TempDir(), testConfig())
	created := uploadOK(t, srv.URL, "locked.txt", "secret", map[string]string{"password": "hunter2"})
	id := created["id"].(string)

	limited := false
	for i := 0; i < 15; i++ {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/d/"+id, nil)
		req.Header.Set("X-File-Password", "wrong")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("guessing was never rate limited")
	}
}

func TestIndexOffersTheSource(t *testing.T) {
	cfg := testConfig()
	cfg.SourceURL = "https://example.org/my-fork"
	srv := newTestServer(t, t.TempDir(), cfg)

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()

	page := string(body)
	if !strings.Contains(page, cfg.SourceURL) {
		t.Error("the page does not link the configured source; AGPL section 13 requires offering it")
	}
	if strings.Contains(page, "__SOURCE_URL__") {
		t.Error("the placeholder was served raw")
	}
	if !strings.Contains(page, "AGPL") {
		t.Error("the page does not name the licence")
	}
}
