package convertx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeCX struct {
	mu            sync.Mutex
	email         string
	password      string
	registered    bool
	registerCalls int
	loginCalls    int
	uploadCalls   int
	polls         int
	rejectToken   string
	resultsDelay  int
	resultsFail   bool
	outputName    string
	outputData    []byte
	loginFailCode int
	uploadDeclare int64
	uploadActual  int64
	uploadContent string
}

func newFakeCX(email, password string) *fakeCX {
	return &fakeCX{
		email:         email,
		password:      password,
		outputName:    "out.mp3",
		outputData:    []byte("converted-data"),
		loginFailCode: http.StatusForbidden,
	}
}

// src is a re-openable source over in-memory bytes, like the store hands the
// client a file on disk.
func srcOf(data string) Source {
	return Source{
		Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(data)), nil
		},
		Size: int64(len(data)),
	}
}

func (f *fakeCX) token() string {
	return fmt.Sprintf("tok-%d", f.loginCalls)
}

func (f *fakeCX) validAuth(r *http.Request) bool {
	c, err := r.Cookie("auth")
	if err != nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rejectToken != "" && c.Value == f.rejectToken {
		return false
	}
	return true
}

func (f *fakeCX) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/login" && r.Method == http.MethodPost:
		var creds struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&creds)
		f.mu.Lock()
		ok := f.registered && creds.Email == f.email && creds.Password == f.password
		f.loginCalls++
		tok := f.token()
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(f.loginFailCode)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "auth", Value: tok})
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusFound)

	case r.URL.Path == "/register" && r.Method == http.MethodPost:
		var creds struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&creds)
		f.mu.Lock()
		f.registerCalls++
		if f.registered {
			f.mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.registered = true
		f.loginCalls++
		tok := f.token()
		f.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "auth", Value: tok})
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusFound)

	case r.URL.Path == "/" && r.Method == http.MethodGet:
		if !f.validAuth(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "jobId", Value: "9"})
		w.Write([]byte("<html><body>convert</body></html>"))

	case r.URL.Path == "/upload" && r.Method == http.MethodPost:
		if !f.validAuth(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		declared := r.ContentLength
		raw, _ := io.ReadAll(r.Body)
		content := ""
		if _, params, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil {
			mr := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
			if part, err := mr.NextPart(); err == nil {
				b, _ := io.ReadAll(part)
				content = string(b)
			}
		}
		f.mu.Lock()
		f.uploadCalls++
		f.uploadDeclare = declared
		f.uploadActual = int64(len(raw))
		f.uploadContent = content
		f.mu.Unlock()
		w.Write([]byte(`{"message":"ok"}`))

	case r.URL.Path == "/convert" && r.Method == http.MethodPost:
		if !f.validAuth(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Location", "/results/9")
		w.WriteHeader(http.StatusFound)

	case strings.HasPrefix(r.URL.Path, "/results/"):
		if !f.validAuth(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.polls++
		empty := f.polls <= f.resultsDelay
		status := "Done"
		if f.resultsFail {
			status = "Failed, check logs"
		}
		row := ""
		if !empty {
			row = fmt.Sprintf(
				`<table><tr><td safe class="x">%s</td><td safe>%s</td>`+
					`<td><a href="/download/1/9/%s">d</a></td></tr></table>`,
				f.outputName, status, f.outputName)
		}
		f.mu.Unlock()
		w.Write([]byte(row))

	case strings.HasPrefix(r.URL.Path, "/download/"):
		if !f.validAuth(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		data := f.outputData
		f.mu.Unlock()
		w.Write(data)

	default:
		http.NotFound(w, r)
	}
}

func newClient(t *testing.T, fx *fakeCX) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(fx.serve))
	t.Cleanup(srv.Close)
	c := New(srv.URL, fx.email, fx.password, 5*time.Second)
	c.pollWait = time.Millisecond
	return c, srv
}

func TestConvertAutoRegister(t *testing.T) {
	fx := newFakeCX("a@b.c", "pass")
	c, _ := newClient(t, fx)

	var got bytes.Buffer
	name, err := c.Convert(context.Background(), srcOf("data"), "in.mp4", "mp3", "", &got)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if name != "out.mp3" {
		t.Errorf("Name = %q, want out.mp3", name)
	}
	if got.String() != "converted-data" {
		t.Errorf("Data = %q", got.String())
	}

	fx.mu.Lock()
	registered := fx.registered
	regCalls := fx.registerCalls
	upCalls := fx.uploadCalls
	fx.mu.Unlock()
	if !registered {
		t.Errorf("account was not registered")
	}
	if regCalls != 1 {
		t.Errorf("register calls = %d, want 1", regCalls)
	}
	if upCalls != 1 {
		t.Errorf("upload calls = %d, want 1", upCalls)
	}
}

func TestConvertExistingAccount(t *testing.T) {
	fx := newFakeCX("a@b.c", "pass")
	fx.registered = true
	c, _ := newClient(t, fx)

	if _, err := c.Convert(context.Background(), srcOf("data"), "in.mp4", "mp3", "", io.Discard); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	fx.mu.Lock()
	regCalls := fx.registerCalls
	fx.mu.Unlock()
	if regCalls != 0 {
		t.Errorf("register calls = %d, want 0 (account already exists)", regCalls)
	}
}

func TestConvertReauthOnExpiredToken(t *testing.T) {
	fx := newFakeCX("a@b.c", "pass")
	fx.registered = true
	fx.rejectToken = "tok-1"
	c, _ := newClient(t, fx)

	var got bytes.Buffer
	if _, err := c.Convert(context.Background(), srcOf("data"), "in.mp4", "mp3", "", &got); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if got.String() != "converted-data" {
		t.Errorf("Data = %q", got.String())
	}
	fx.mu.Lock()
	logins := fx.loginCalls
	fx.mu.Unlock()
	if logins < 2 {
		t.Errorf("login calls = %d, want at least 2 (re-auth)", logins)
	}
}

// The real ConvertX answers an unknown account with 403, which is what makes
// ensureAuth fall through to registration. A 401 must register too: relying on
// one status code silently disables auto-registration if it ever changes.
func TestConvertAutoRegisterOn401(t *testing.T) {
	fx := newFakeCX("a@b.c", "pass")
	fx.loginFailCode = http.StatusUnauthorized
	c, _ := newClient(t, fx)

	var got bytes.Buffer
	if _, err := c.Convert(context.Background(), srcOf("data"), "in.mp4", "mp3", "", &got); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	fx.mu.Lock()
	registered, regCalls := fx.registered, fx.registerCalls
	fx.mu.Unlock()
	if !registered || regCalls != 1 {
		t.Errorf("registered = %v, register calls = %d; want true, 1", registered, regCalls)
	}
	if got.String() != "converted-data" {
		t.Errorf("Data = %q", got.String())
	}
}

// The upload streams from a pipe, so its length is computed rather than
// measured. If that arithmetic drifts the request is either truncated or hangs.
func TestUploadDeclaresExactLength(t *testing.T) {
	fx := newFakeCX("a@b.c", "pass")
	fx.registered = true
	c, _ := newClient(t, fx)

	payload := strings.Repeat("x", 100_000)
	if _, err := c.Convert(context.Background(), srcOf(payload), "in.mp4", "mp3", "", io.Discard); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	fx.mu.Lock()
	declared, actual, content := fx.uploadDeclare, fx.uploadActual, fx.uploadContent
	fx.mu.Unlock()

	if declared < 0 {
		t.Fatalf("no Content-Length sent (chunked); the computed length was not applied")
	}
	if declared != actual {
		t.Errorf("Content-Length = %d, body was %d bytes", declared, actual)
	}
	if content != payload {
		t.Errorf("uploaded %d bytes of content, want %d", len(content), len(payload))
	}
}

func TestConvertUnsupportedTarget(t *testing.T) {
	fx := newFakeCX("a@b.c", "pass")
	c, _ := newClient(t, fx)

	if _, err := c.Convert(context.Background(), srcOf("data"), "in.mp4", "xyzzy", "", io.Discard); !isUnsupported(err) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	fx.mu.Lock()
	calls := fx.loginCalls + fx.registerCalls
	fx.mu.Unlock()
	if calls != 0 {
		t.Errorf("network calls = %d, unsupported target should fail before any request", calls)
	}
}

func TestConvertFailureStatus(t *testing.T) {
	fx := newFakeCX("a@b.c", "pass")
	fx.registered = true
	fx.resultsFail = true
	c, _ := newClient(t, fx)

	if _, err := c.Convert(context.Background(), srcOf("data"), "in.mp4", "mp3", "", io.Discard); !isConvertFailed(err) {
		t.Fatalf("err = %v, want ErrConvertFailed", err)
	}
}

func TestConvertTimeout(t *testing.T) {
	fx := newFakeCX("a@b.c", "pass")
	fx.registered = true
	fx.resultsDelay = 1 << 30
	c, _ := newClient(t, fx)
	c.timeout = 50 * time.Millisecond
	c.pollWait = 5 * time.Millisecond

	if _, err := c.Convert(context.Background(), srcOf("data"), "in.mp4", "mp3", "", io.Discard); !errors_IsTimeout(err) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}

func TestParseResults(t *testing.T) {
	html := `<table><tr><td safe class="x">a.mp3</td><td safe>Done</td><td><a href="/download/1/9/a.mp3">d</a></td></tr></table>`
	rows := parseResults([]byte(html))
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.name != "a.mp3" || r.status != "Done" {
		t.Errorf("row = %+v", r)
	}
	if !strings.Contains(r.href, "/download/1/9/a.mp3") {
		t.Errorf("href = %q", r.href)
	}
}

func TestParseResultsPendingRow(t *testing.T) {
	html := `<table><tr><td safe>pending</td></tr></table>`
	rows := parseResults([]byte(html))
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 pending row", len(rows))
	}
	if rows[0].href != "" || rows[0].status != "pending" {
		t.Errorf("row = %+v, want pending row without download link", rows[0])
	}
}

func isUnsupported(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "no converter") || strings.Contains(err.Error(), ErrUnsupported.Error()))
}

func isConvertFailed(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrConvertFailed.Error())
}

func errors_IsTimeout(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrTimeout.Error())
}
