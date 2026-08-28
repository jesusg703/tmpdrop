package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jesusg703/tmpdrop/internal/convertx"
	"github.com/jesusg703/tmpdrop/internal/store"
)

var validID = regexp.MustCompile(`^[0-9a-f]{24}$`)

type formatInfo struct {
	Ext       string `json:"ext"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Protected bool   `json:"protected"`
}

type fileDTO struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Size             int64             `json:"size"`
	Mime             string            `json:"mime,omitempty"`
	Note             string            `json:"note,omitempty"`
	UploadedAt       time.Time         `json:"uploaded_at"`
	ExpiresAt        time.Time         `json:"expires_at,omitempty"`
	Expired          bool              `json:"expired"`
	Downloads        int               `json:"downloads"`
	MaxDownloads     int               `json:"max_downloads"`
	URL              string            `json:"url"`
	DownloadURL      string            `json:"download_url"`
	Ext              string            `json:"ext"`
	Protected        bool              `json:"protected"`
	Formats          []formatInfo      `json:"formats"`
	Targets          []convertx.Target `json:"targets"`
	ConvertAvailable bool              `json:"convert_available"`
}

func extOf(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}

func (s *Server) toDTO(f *store.File) fileDTO {
	ext := extOf(f.Name)
	available := []formatInfo{{
		Ext:       ext,
		Name:      f.Name,
		URL:       "/d/" + f.ID,
		Protected: f.PasswordHash != "",
	}}

	produced := map[string]bool{}
	totalDownloads := f.Downloads
	if f.DerivedFrom == "" {
		for _, d := range s.store.Derived(f.ID) {
			dext := extOf(d.Name)
			if dext == "" || produced[dext] {
				continue
			}
			produced[dext] = true
			available = append(available, formatInfo{
				Ext:       dext,
				Name:      d.Name,
				URL:       "/d/" + d.ID,
				Protected: d.PasswordHash != "",
			})
			totalDownloads += d.Downloads
		}
	}

	var targets []convertx.Target
	if f.DerivedFrom == "" {
		for _, t := range convertx.PossibleTargets(f.Name) {
			if t.Ext == ext || produced[t.Ext] {
				continue
			}
			targets = append(targets, t)
		}
	}

	return fileDTO{
		ID:               f.ID,
		Name:             f.Name,
		Size:             f.Size,
		Mime:             f.Mime,
		Note:             f.Note,
		UploadedAt:       f.UploadedAt,
		ExpiresAt:        f.ExpiresAt,
		Expired:          f.Expired(time.Now()),
		Downloads:        totalDownloads,
		MaxDownloads:     f.MaxDownloads,
		URL:              "/d/" + f.ID + "?inline=1",
		DownloadURL:      "/d/" + f.ID,
		Ext:              ext,
		Protected:        f.PasswordHash != "",
		Formats:          available,
		Targets:          targets,
		ConvertAvailable: s.conv != nil,
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(s.index)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("ok"))
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxFileSize+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	var (
		tmpPath  string
		fileName string
		mimeType string
		fields   = map[string]string{}
	)
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeUploadError(w, err)
			return
		}
		name := part.FormName()

		if name == "file" && part.FileName() != "" && tmpPath == "" {
			fileName = part.FileName()
			tmpPath, mimeType, err = s.spool(part)
			part.Close()
			if err != nil {
				writeUploadError(w, err)
				return
			}
			continue
		}

		value, err := io.ReadAll(io.LimitReader(part, 1<<16))
		part.Close()
		if err != nil {
			writeUploadError(w, err)
			return
		}
		fields[name] = string(value)
	}

	if tmpPath == "" {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}

	opt := store.AddOptions{
		ClientKey: firstNonEmpty(fields["key"], s.clientIP(r)),
		Note:      strings.TrimSpace(fields["note"]),
		Password:  fields["password"],
	}
	if dl := fields["max_downloads"]; dl != "" {
		n, err := strconv.Atoi(dl)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid max_downloads")
			return
		}
		opt.MaxDownloads = n
	}
	if opt.TTL, err = parseTTL(fields["ttl"]); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := s.store.Adopt(tmpPath, fileName, mimeType, opt)
	if err != nil {
		writeStoreWriteError(w, s.log, err)
		return
	}
	tmpPath = ""

	if wantsJSON(r) {
		writeJSON(w, http.StatusCreated, s.toDTO(created))
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) spool(part io.Reader) (path, mimeType string, err error) {
	tmp, err := s.store.TempFile()
	if err != nil {
		return "", "", err
	}
	path = tmp.Name()

	peek := make([]byte, 512)
	n, err := io.ReadFull(part, peek)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		tmp.Close()
		_ = os.Remove(path)
		return "", "", err
	}
	mimeType = http.DetectContentType(peek[:n])

	if _, err := tmp.Write(peek[:n]); err != nil {
		tmp.Close()
		_ = os.Remove(path)
		return "", "", err
	}
	if _, err := io.Copy(tmp, part); err != nil {
		tmp.Close()
		_ = os.Remove(path)
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", "", err
	}
	return path, mimeType, nil
}

// sniff types an already-written file from its first bytes. ReadAt leaves the
// write offset alone, so the caller can keep appending afterwards.
func sniff(f *os.File) (string, error) {
	peek := make([]byte, 512)
	n, err := f.ReadAt(peek, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return http.DetectContentType(peek[:n]), nil
}

func writeUploadError(w http.ResponseWriter, err error) {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		writeError(w, http.StatusRequestEntityTooLarge, "upload too large")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid multipart form")
}

func writeStoreWriteError(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, store.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the size limit")
	case errors.Is(err, store.ErrEmpty):
		writeError(w, http.StatusBadRequest, "empty file")
	case errors.Is(err, store.ErrStorageFull):
		writeError(w, http.StatusInsufficientStorage, "storage is full")
	case errors.Is(err, store.ErrQuotaExceeded):
		writeError(w, http.StatusForbidden, "client quota exceeded")
	case errors.Is(err, store.ErrFileLimit):
		writeError(w, http.StatusForbidden, "client file limit reached")
	default:
		log.Error("upload failed", "error", err)
		writeError(w, http.StatusInternalServerError, "upload failed")
	}
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID.MatchString(id) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	protected, err := s.store.Protected(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if protected {
		if !s.pwLimit.allow(s.clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many password attempts")
			return
		}
		password := r.Header.Get("X-File-Password")
		_, ok, err := s.store.VerifyPassword(id, password)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if !ok {
			if password == "" {
				writeError(w, http.StatusUnauthorized, "password required")
			} else {
				writeError(w, http.StatusForbidden, "incorrect password")
			}
			return
		}
		s.pwLimit.reset(s.clientIP(r))
	}

	f, blob, err := s.store.OpenBlob(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	defer blob.Close()

	mimeType := f.Mime
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	disposition := "attachment"
	if r.URL.Query().Has("inline") && inlineSafe(mimeType) {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")

	http.ServeContent(w, r, f.Name, time.Time{}, blob)

	if r.Method == http.MethodHead || r.Header.Get("Range") != "" {
		return
	}
	exhausted, err := s.store.RecordDownload(id)
	if err != nil {
		s.log.Warn("download not counted", "file", id, "error", err)
		return
	}
	if exhausted {
		if err := s.store.Delete(id); err != nil {
			s.log.Warn("spent file not removed", "file", id, "error", err)
		}
	}
}

func inlineSafe(mimeType string) bool {
	base, _, err := mime.ParseMediaType(mimeType)
	if err != nil || base == "image/svg+xml" {
		return false
	}
	if base == "application/pdf" || base == "text/plain" {
		return true
	}
	return strings.HasPrefix(base, "image/") ||
		strings.HasPrefix(base, "video/") ||
		strings.HasPrefix(base, "audio/")
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	files := s.store.List()
	used, count := s.store.Stats()
	dtos := make([]fileDTO, 0, len(files))
	for _, f := range files {
		if f.DerivedFrom != "" {
			continue
		}
		dtos = append(dtos, s.toDTO(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"files":       dtos,
		"used":        used,
		"count":       count,
		"max_storage": s.cfg.MaxStorage,
	})
}

func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := s.store.Get(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toDTO(f))
}

func (s *Server) handleFileTargets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := s.store.Get(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	targets := convertx.TargetsFor(f.Name)
	if targets == nil {
		targets = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"targets": targets})
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		s.log.Error("delete failed", "error", err)
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	s.convMu.Lock()
	delete(s.convJobs, id)
	s.convMu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConvertStart(w http.ResponseWriter, r *http.Request) {
	if s.conv == nil {
		writeError(w, http.StatusServiceUnavailable, "conversion service is not configured")
		return
	}
	id := r.PathValue("id")
	if _, err := s.store.Get(id); err != nil {
		writeStoreError(w, err)
		return
	}

	var body struct {
		Target    string `json:"target"`
		Converter string `json:"converter"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	target := strings.ToLower(strings.TrimSpace(body.Target))
	if !convertx.Supported(target) {
		writeError(w, http.StatusBadRequest, "unsupported target format")
		return
	}
	converter := strings.TrimSpace(body.Converter)
	if converter != "" {
		if _, ok := convertx.ConverterSpecs()[converter]; !ok {
			writeError(w, http.StatusBadRequest, "unknown converter")
			return
		}
	}

	s.convMu.Lock()
	s.purgeOldJobsLocked()
	if prev, exists := s.convJobs[id]; exists && prev.Status == statusRunning {
		s.convMu.Unlock()
		writeError(w, http.StatusConflict, "a conversion is already running for this file")
		return
	}
	job := &convJob{SourceID: id, Target: target, Converter: converter, Status: statusRunning}
	s.convJobs[id] = job
	s.convMu.Unlock()

	go s.runConversion(id, target, converter)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": statusRunning, "target": target})
}

func (s *Server) handleConvertStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.convMu.Lock()
	job, ok := s.convJobs[id]
	if !ok {
		s.convMu.Unlock()
		writeError(w, http.StatusNotFound, "no conversion for this file")
		return
	}
	resp := struct {
		Status    string `json:"status"`
		Target    string `json:"target"`
		Message   string `json:"message,omitempty"`
		Name      string `json:"name,omitempty"`
		NewID     string `json:"new_id,omitempty"`
		URL       string `json:"url,omitempty"`
		Protected bool   `json:"protected"`
	}{
		Status:  job.Status,
		Target:  job.Target,
		Message: job.Message,
	}
	if job.Status == statusDone && job.NewID != "" {
		if f, err := s.store.Get(job.NewID); err == nil {
			resp.Name = f.Name
			resp.NewID = f.ID
			resp.URL = "/d/" + f.ID
			resp.Protected = f.PasswordHash != ""
		}
	}
	s.convMu.Unlock()
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) runConversion(id, target, converter string) {
	src, err := s.store.Get(id)
	if err != nil {
		s.finishConv(id, statusError, err.Error())
		return
	}
	blobPath, err := s.store.BlobPath(id)
	if err != nil {
		s.finishConv(id, statusError, err.Error())
		return
	}

	tmp, err := s.store.TempFile()
	if err != nil {
		s.finishConv(id, statusError, "could not stage the conversion result")
		return
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ConvertX.Timeout)
	defer cancel()

	source := convertx.Source{
		Open: func() (io.ReadCloser, error) { return os.Open(blobPath) },
		Size: src.Size,
	}
	name, err := s.conv.Convert(ctx, source, src.Name, target, converter, tmp)
	if err != nil {
		s.finishConv(id, statusError, err.Error())
		return
	}

	mimeType, err := sniff(tmp)
	if err != nil {
		s.finishConv(id, statusError, "could not read the conversion result")
		return
	}
	if err := tmp.Close(); err != nil {
		s.finishConv(id, statusError, "could not write the conversion result")
		return
	}

	var ttl time.Duration
	if src.ExpiresAt.IsZero() {
		ttl = -1
	} else if rem := time.Until(src.ExpiresAt); rem > 0 {
		ttl = rem
	}

	newFile, err := s.store.Adopt(tmpPath, name, mimeType, store.AddOptions{
		ClientKey:    src.ClientKey,
		Note:         "Converted from " + src.Name,
		TTL:          ttl,
		PasswordHash: src.PasswordHash,
		DerivedFrom:  src.ID,
	})
	if err != nil {
		if errors.Is(err, store.ErrEmpty) {
			s.finishConv(id, statusError, "the converter produced an empty file")
			return
		}
		s.log.Warn("conversion result not stored", "file", id, "error", err)
		s.finishConv(id, statusError, "converted, but the result could not be stored")
		return
	}
	tmpPath = ""
	s.finishConv(id, statusDone, "", newFile.ID)
}

func (s *Server) finishConv(id, status, msg string, newID ...string) {
	s.convMu.Lock()
	defer s.convMu.Unlock()
	if job, ok := s.convJobs[id]; ok {
		job.Status = status
		job.Message = msg
		job.Finished = time.Now()
		if len(newID) > 0 {
			job.NewID = newID[0]
		}
	}
}

func (s *Server) purgeOldJobsLocked() {
	cutoff := time.Now().Add(-jobRetention)
	for id, job := range s.convJobs {
		if job.Status == statusRunning || job.Finished.After(cutoff) {
			continue
		}
		delete(s.convJobs, id)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "file not found")
	case errors.Is(err, store.ErrExpired):
		writeError(w, http.StatusGone, "file expired")
	case errors.Is(err, store.ErrDownloadsUsed):
		writeError(w, http.StatusGone, "download limit reached")
	default:
		slog.Error("store error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") || strings.HasPrefix(r.URL.Path, "/api/")
}

func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !s.trusts(host) {
		return host
	}
	fwd := r.Header.Get("X-Forwarded-For")
	if fwd == "" {
		return host
	}
	if first, _, found := strings.Cut(fwd, ","); found {
		fwd = first
	}
	if fwd = strings.TrimSpace(fwd); fwd != "" && net.ParseIP(fwd) != nil {
		return fwd
	}
	return host
}

func (s *Server) trusts(host string) bool {
	if len(s.trustedProxies) == 0 {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range s.trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseTTL(raw string) (time.Duration, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "", "default":
		return 0, nil
	case "0", "never", "none", "forever":
		return -1, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid expiration %q", raw)
	}
	return d, nil
}
