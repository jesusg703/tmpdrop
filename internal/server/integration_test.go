package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jesusg703/tmpdrop/internal/config"
)

type miniCX struct {
	mu         sync.Mutex
	registered bool
	outputName string
	output     []byte
}

func (m *miniCX) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/login" && r.Method == http.MethodPost:
		m.mu.Lock()
		ok := m.registered
		m.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "auth", Value: "tok-1"})
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusFound)

	case r.URL.Path == "/register" && r.Method == http.MethodPost:
		m.mu.Lock()
		m.registered = true
		m.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "auth", Value: "tok-1"})
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusFound)

	case r.URL.Path == "/" && r.Method == http.MethodGet:
		if _, err := r.Cookie("auth"); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "jobId", Value: "7"})
		w.Write([]byte("<html></html>"))

	case r.URL.Path == "/upload" && r.Method == http.MethodPost:
		if _, err := r.Cookie("auth"); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"message":"ok"}`))

	case r.URL.Path == "/convert" && r.Method == http.MethodPost:
		if _, err := r.Cookie("auth"); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Location", "/results/7")
		w.WriteHeader(http.StatusFound)

	case strings.HasPrefix(r.URL.Path, "/results/"):
		if _, err := r.Cookie("auth"); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		m.mu.Lock()
		name := m.outputName
		m.mu.Unlock()
		w.Write([]byte(fmt.Sprintf(
			`<table><tr><td safe>%s</td><td safe>Done</td><td><a href="/download/1/7/%s">d</a></td></tr></table>`,
			name, name)))

	case strings.HasPrefix(r.URL.Path, "/download/"):
		if _, err := r.Cookie("auth"); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		m.mu.Lock()
		out := m.output
		m.mu.Unlock()
		w.Write(out)

	default:
		http.NotFound(w, r)
	}
}

func TestEndToEndConvertFlow(t *testing.T) {
	cx := &miniCX{outputName: "out.mp3", output: []byte("converted-by-fake")}
	cxSrv := httptest.NewServer(http.HandlerFunc(cx.serve))
	defer cxSrv.Close()

	cfg := testConfig()
	cfg.ConvertX = config.ConvertX{
		URL:      cxSrv.URL,
		Email:    "tmpdrop@example.com",
		Password: "tmpdrop-demo",
		Timeout:  5 * time.Second,
	}
	srv := newTestServer(t, t.TempDir(), cfg)

	created := uploadOK(t, srv.URL, "clip.mp4", "source-video", nil)
	srcID := created["id"].(string)

	body := strings.NewReader(`{"target":"mp3"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/files/"+srcID+"/convert", body)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("convert status = %d, want 202", res.StatusCode)
	}

	deadline := time.Now().Add(5 * time.Second)
	var status struct {
		Status  string `json:"status"`
		NewID   string `json:"new_id"`
		Name    string `json:"name"`
		Message string `json:"message"`
		URL     string `json:"url"`
	}
	for {
		res, err := http.Get(srv.URL + "/api/convert/" + srcID)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
			res.Body.Close()
			t.Fatal(err)
		}
		res.Body.Close()
		if status.Status == "done" || status.Status == "error" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("conversion did not finish in time; last status: %+v", status)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if status.Status != "done" {
		t.Fatalf("conversion status = %q, message %q", status.Status, status.Message)
	}
	if status.Name != "out.mp3" || status.NewID == "" || status.NewID == srcID {
		t.Errorf("unexpected outcome: %+v", status)
	}

	res, err = http.Get(srv.URL + "/d/" + status.NewID)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(out) != "converted-by-fake" {
		t.Errorf("converted download = %d %q", res.StatusCode, out)
	}

	res, err = http.Get(srv.URL + "/d/" + srcID)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("source download = %d", res.StatusCode)
	}

	res, err = http.Get(srv.URL + "/api/files")
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Count int `json:"count"`
		Files []struct {
			Formats []struct {
				Ext string `json:"ext"`
			} `json:"formats"`
		} `json:"files"`
	}
	json.NewDecoder(res.Body).Decode(&list)
	res.Body.Close()
	if list.Count != 1 {
		t.Errorf("file count = %d, want 1", list.Count)
	}
	if len(list.Files) != 1 || len(list.Files[0].Formats) != 2 {
		t.Errorf("expected 1 file with 2 formats, got %d files", len(list.Files))
	}
}

func TestConvertUnsupportedTargetOverHTTP(t *testing.T) {
	cfg := testConfig()
	cfg.ConvertX = config.ConvertX{URL: "http://127.0.0.1:1", Email: "a", Password: "b", Timeout: time.Second}
	srv := newTestServer(t, t.TempDir(), cfg)

	created := uploadOK(t, srv.URL, "a.mp4", "x", nil)
	id := created["id"].(string)

	body := strings.NewReader(`{"target":"zzz"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/files/"+id+"/convert", body)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", res.StatusCode)
	}
}

func TestSecondConversionOfSameFileIsAccepted(t *testing.T) {
	cx := &miniCX{outputName: "out.mp3", output: []byte("converted-by-fake")}
	cxSrv := httptest.NewServer(http.HandlerFunc(cx.serve))
	defer cxSrv.Close()

	cfg := testConfig()
	cfg.ConvertX = config.ConvertX{
		URL:      cxSrv.URL,
		Email:    "tmpdrop@example.com",
		Password: "tmpdrop-demo",
		Timeout:  5 * time.Second,
	}
	srv := newTestServer(t, t.TempDir(), cfg)

	created := uploadOK(t, srv.URL, "clip.mp4", "source-video", nil)
	srcID := created["id"].(string)

	for i, target := range []string{"mp3", "wav"} {
		if code := startConvert(t, srv.URL, srcID, target); code != http.StatusAccepted {
			t.Fatalf("conversion %d to %s = %d, want 202", i+1, target, code)
		}
		if st := awaitConversion(t, srv.URL, srcID); st != "done" {
			t.Fatalf("conversion %d to %s finished as %q", i+1, target, st)
		}
	}
}

func startConvert(t *testing.T, base, id, target string) int {
	t.Helper()
	body := strings.NewReader(fmt.Sprintf(`{"target":%q}`, target))
	req, _ := http.NewRequest(http.MethodPost, base+"/api/files/"+id+"/convert", body)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	return res.StatusCode
}

func awaitConversion(t *testing.T, base, id string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err := http.Get(base + "/api/convert/" + id)
		if err != nil {
			t.Fatal(err)
		}
		var status struct {
			Status string `json:"status"`
		}
		err = json.NewDecoder(res.Body).Decode(&status)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if status.Status != "running" {
			return status.Status
		}
		if time.Now().After(deadline) {
			t.Fatal("conversion did not finish in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
