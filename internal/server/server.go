package server

import (
	"bytes"
	"html"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jesusg703/tmpdrop/internal/config"
	"github.com/jesusg703/tmpdrop/internal/convertx"
	"github.com/jesusg703/tmpdrop/internal/server/web"
	"github.com/jesusg703/tmpdrop/internal/store"
)

func init() {
	_ = mime.AddExtensionType(".woff2", "font/woff2")
}

const (
	statusRunning = "running"
	statusDone    = "done"
	statusError   = "error"
)

const jobRetention = 10 * time.Minute

type Server struct {
	store *store.Store
	cfg   config.Config
	conv  *convertx.Client
	log   *slog.Logger

	index          []byte
	pwLimit        *pwAttempts
	trustedProxies []*net.IPNet

	convMu   sync.Mutex
	convJobs map[string]*convJob
}

type convJob struct {
	SourceID  string
	Target    string
	Converter string
	Status    string
	Message   string
	NewID     string
	Finished  time.Time
}

func New(st *store.Store, cfg config.Config, log *slog.Logger) *Server {
	s := &Server{
		store:    st,
		cfg:      cfg,
		log:      log,
		convJobs: map[string]*convJob{},
		pwLimit:  newPWAttempts(10, time.Minute),
	}
	s.index = buildIndex(cfg.SourceURL, log)
	for _, cidr := range cfg.TrustedProxies {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			s.trustedProxies = append(s.trustedProxies, n)
		} else if ip := net.ParseIP(cidr); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			s.trustedProxies = append(s.trustedProxies, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
		} else {
			log.Warn("ignoring unparsable trusted proxy", "value", cidr)
		}
	}
	if cfg.ConvertX.Enabled() {
		s.conv = convertx.New(cfg.ConvertX.URL, cfg.ConvertX.Email, cfg.ConvertX.Password, cfg.ConvertX.Timeout)
	}
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /upload", s.handleUpload)
	mux.HandleFunc("GET /d/{id}", s.handleDownload)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/files", s.handleListFiles)
	mux.HandleFunc("POST /api/upload", s.handleUpload)
	mux.HandleFunc("GET /api/files/{id}", s.handleGetFile)
	mux.HandleFunc("GET /api/files/{id}/targets", s.handleFileTargets)
	mux.HandleFunc("DELETE /api/files/{id}", s.handleDeleteFile)
	mux.HandleFunc("POST /api/files/{id}/convert", s.handleConvertStart)
	mux.HandleFunc("GET /api/convert/{id}", s.handleConvertStatus)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(web.FS))))
	return s.withMiddleware(mux)
}

func (s *Server) withMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				s.log.Error("panic recovered", "panic", p)
				if !rec.wrote {
					http.Error(rec, "internal error", http.StatusInternalServerError)
					rec.status = http.StatusInternalServerError
				}
			}
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				s.log.Debug("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "dur", time.Since(start).Round(time.Microsecond))
			} else {
				s.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "dur", time.Since(start).Round(time.Microsecond))
			}
		}()
		setSecurityHeaders(rec)
		h.ServeHTTP(rec, r)
	})
}

func setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self'; "+
			"style-src 'self'; "+
			"font-src 'self'; "+
			"img-src 'self' data:; "+
			"connect-src 'self'; "+
			"frame-ancestors 'none'; "+
			"base-uri 'self'; "+
			"form-action 'self'")
}

type recorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *recorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.status = http.StatusOK
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func buildIndex(sourceURL string, log *slog.Logger) []byte {
	data, err := web.FS.ReadFile("index.html")
	if err != nil {
		log.Error("embedded index missing", "error", err)
		return []byte("<!doctype html><title>tmpdrop</title><p>internal error")
	}
	if sourceURL == "" {
		sourceURL = config.DefaultSourceURL
	}
	return bytes.ReplaceAll(data, []byte("__SOURCE_URL__"), []byte(html.EscapeString(sourceURL)))
}
