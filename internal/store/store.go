package store

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	manifestName  = "manifest.json"
	blobDirName   = "files"
	manifestVer   = 1
	maxNameLength = 200
	cleanName     = "file"
	tempPrefix    = "upload-"
	tempMaxAge    = time.Hour
	reclaimGrace  = time.Hour
)

var (
	ErrNotFound      = errors.New("store: file not found")
	ErrExpired       = errors.New("store: file expired")
	ErrDownloadsUsed = errors.New("store: download limit reached")
	ErrTooLarge      = errors.New("store: file exceeds maximum size")
	ErrEmpty         = errors.New("store: empty upload")
	ErrStorageFull   = errors.New("store: storage limit reached")
	ErrQuotaExceeded = errors.New("store: client quota exceeded")
	ErrFileLimit     = errors.New("store: client file limit reached")
)

type File struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	Mime         string    `json:"mime"`
	Note         string    `json:"note"`
	ClientKey    string    `json:"client_key,omitempty"`
	UploadedAt   time.Time `json:"uploaded_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	MaxDownloads int       `json:"max_downloads"`
	Downloads    int       `json:"downloads"`
	PasswordHash string    `json:"password_hash,omitempty"`
	DerivedFrom  string    `json:"derived_from,omitempty"`
}

func (f *File) Expired(now time.Time) bool {
	return !f.ExpiresAt.IsZero() && now.After(f.ExpiresAt)
}

func (f *File) clone() *File {
	c := *f
	return &c
}

type Limits struct {
	MaxFileSize    int64
	MaxStorage     int64
	DefaultTTL     time.Duration
	QuotaDefault   int64
	MaxFilesClient int
}

type AddOptions struct {
	ClientKey    string
	Note         string
	TTL          time.Duration
	MaxDownloads int
	Password     string
	PasswordHash string
	DerivedFrom  string
	Now          time.Time
}

type Store struct {
	mu             sync.RWMutex
	root           string
	blobs          string
	manifestPath   string
	files          map[string]*File
	limits         Limits
	reclaimBlocked bool
}

func Open(dir string, limits Limits) (*Store, error) {
	if dir == "" {
		return nil, errors.New("store: empty root directory")
	}
	blobs := filepath.Join(dir, blobDirName)
	for _, d := range []string{dir, blobs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("store: create directory: %w", err)
		}
	}
	s := &Store{
		root:         dir,
		blobs:        blobs,
		manifestPath: filepath.Join(dir, manifestName),
		files:        map[string]*File{},
		limits:       limits,
	}
	if err := s.loadManifest(); err != nil {
		return nil, err
	}
	s.purgeOrphans()
	return s, nil
}

func (s *Store) Add(r io.Reader, name string, mime string, opt AddOptions) (*File, error) {
	tmp, err := s.TempFile()
	if err != nil {
		return nil, err
	}
	path := tmp.Name()

	_, copyErr := io.Copy(tmp, io.LimitReader(r, s.limits.MaxFileSize+1))
	closeErr := tmp.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("store: write blob: %w", copyErr)
	}

	f, err := s.Adopt(path, name, mime, opt)
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return f, nil
}

func (s *Store) TempFile() (*os.File, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, fmt.Errorf("store: temp name: %w", err)
	}
	path := filepath.Join(s.root, tempPrefix+hex.EncodeToString(b[:]))
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, fmt.Errorf("store: create temp: %w", err)
	}
	return f, nil
}

func (s *Store) Adopt(tmpPath, name, mime string, opt AddOptions) (*File, error) {
	info, err := os.Stat(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("store: stat upload: %w", err)
	}
	written := info.Size()
	if written > s.limits.MaxFileSize {
		return nil, ErrTooLarge
	}
	if written == 0 {
		return nil, ErrEmpty
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	f, err := s.newRecord(id, name, mime, written, opt)
	if err != nil {
		return nil, err
	}
	blobPath := filepath.Join(s.blobs, id)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.checkLimits(f); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, blobPath); err != nil {
		return nil, fmt.Errorf("store: commit upload: %w", err)
	}
	s.files[id] = f
	if err := s.persistManifest(); err != nil {
		delete(s.files, id)
		_ = os.Remove(blobPath)
		return nil, err
	}
	return f, nil
}

func (s *Store) newRecord(id, name, mime string, size int64, opt AddOptions) (*File, error) {
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	key := opt.ClientKey
	if key == "" {
		key = "anonymous"
	}
	f := &File{
		ID:           id,
		Name:         sanitizeName(name),
		Size:         size,
		Mime:         mime,
		Note:         opt.Note,
		ClientKey:    key,
		UploadedAt:   now,
		MaxDownloads: opt.MaxDownloads,
		DerivedFrom:  opt.DerivedFrom,
	}
	switch {
	case opt.PasswordHash != "":
		f.PasswordHash = opt.PasswordHash
	case opt.Password != "":
		h, err := hashPassword(opt.Password)
		if err != nil {
			return nil, err
		}
		f.PasswordHash = h
	}
	switch {
	case opt.TTL < 0:
	case opt.TTL > 0:
		f.ExpiresAt = now.Add(opt.TTL)
	case s.limits.DefaultTTL > 0:
		f.ExpiresAt = now.Add(s.limits.DefaultTTL)
	}
	return f, nil
}

func (s *Store) Get(id string) (*File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.getLocked(id, time.Now())
	if err != nil {
		return nil, err
	}
	return f.clone(), nil
}

func (s *Store) OpenBlob(id string) (*File, *os.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, err := s.getLocked(id, time.Now())
	if err != nil {
		return nil, nil, err
	}
	if f.MaxDownloads > 0 && f.Downloads >= f.MaxDownloads {
		return nil, nil, ErrDownloadsUsed
	}

	blob, err := os.Open(filepath.Join(s.blobs, id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("store: open blob: %w", err)
	}
	return f.clone(), blob, nil
}

func (s *Store) RecordDownload(id string) (exhausted bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.files[id]
	if !ok {
		return false, ErrNotFound
	}
	f.Downloads++
	if err := s.persistManifest(); err != nil {
		f.Downloads--
		return false, fmt.Errorf("store: persist manifest: %w", err)
	}
	return f.MaxDownloads > 0 && f.Downloads >= f.MaxDownloads, nil
}

func (s *Store) BlobPath(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := s.getLocked(id, time.Now()); err != nil {
		return "", err
	}
	return filepath.Join(s.blobs, id), nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[id]
	if !ok {
		return ErrNotFound
	}
	_ = os.Remove(filepath.Join(s.blobs, id))
	delete(s.files, id)
	if f.DerivedFrom == "" {
		for did, d := range s.files {
			if d.DerivedFrom == id {
				_ = os.Remove(filepath.Join(s.blobs, did))
				delete(s.files, did)
			}
		}
	}
	return s.persistManifest()
}

func (s *Store) List() []*File {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*File, 0, len(s.files))
	for _, f := range s.files {
		out = append(out, f.clone())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UploadedAt.After(out[j].UploadedAt)
	})
	return out
}

func (s *Store) Stats() (bytes int64, count int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.files {
		bytes += f.Size
		if f.DerivedFrom == "" {
			count++
		}
	}
	return bytes, count
}

func (s *Store) Sweep(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for id, f := range s.files {
		if f.Expired(now) {
			_ = os.Remove(filepath.Join(s.blobs, id))
			delete(s.files, id)
			removed++
		}
	}
	s.purgeOrphans()
	s.reclaimUnreferenced(now)
	if removed > 0 {
		if err := s.persistManifest(); err != nil {
			_ = err
		}
	}
	return removed
}

func (s *Store) Derived(id string) []*File {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*File
	for _, f := range s.files {
		if f.DerivedFrom == id {
			out = append(out, f.clone())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UploadedAt.Before(out[j].UploadedAt)
	})
	return out
}

func (s *Store) Protected(id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.getLocked(id, time.Now())
	if err != nil {
		return false, err
	}
	return f.PasswordHash != "", nil
}

func (s *Store) VerifyPassword(id, password string) (required, ok bool, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, err := s.getLocked(id, time.Now())
	if err != nil {
		return false, false, err
	}
	if f.PasswordHash == "" {
		return false, true, nil
	}
	ok, err = verifyPassword(f.PasswordHash, password)
	return true, ok, err
}

const (
	pwSaltLen = 16
	pwIter    = 600_000
	pwKeyLen  = 32
	pwMaxLen  = 256
)

func hashPassword(password string) (string, error) {
	if len(password) > pwMaxLen {
		password = password[:pwMaxLen]
	}
	salt := make([]byte, pwSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("store: password salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pwIter, pwKeyLen)
	if err != nil {
		return "", fmt.Errorf("store: password hash: %w", err)
	}
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(key), nil
}

func verifyPassword(encoded, password string) (bool, error) {
	parts := strings.SplitN(encoded, ":", 2)
	if len(parts) != 2 {
		return false, errors.New("store: malformed password hash")
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false, fmt.Errorf("store: decode salt: %w", err)
	}
	want, err := hex.DecodeString(parts[1])
	if err != nil {
		return false, fmt.Errorf("store: decode hash: %w", err)
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, pwIter, pwKeyLen)
	if err != nil {
		return false, fmt.Errorf("store: verify password: %w", err)
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func (s *Store) getLocked(id string, now time.Time) (*File, error) {
	f, ok := s.files[id]
	if !ok {
		return nil, ErrNotFound
	}
	if f.Expired(now) {
		return nil, ErrExpired
	}
	return f, nil
}

func (s *Store) checkLimits(f *File) error {
	var total int64
	for _, cur := range s.files {
		total += cur.Size
	}
	if s.limits.MaxStorage > 0 && total+f.Size > s.limits.MaxStorage {
		return ErrStorageFull
	}
	if s.limits.QuotaDefault > 0 || s.limits.MaxFilesClient > 0 {
		var clientBytes int64
		var clientCount int
		for _, cur := range s.files {
			if cur.ClientKey == f.ClientKey {
				clientBytes += cur.Size
				clientCount++
			}
		}
		if s.limits.QuotaDefault > 0 && clientBytes+f.Size > s.limits.QuotaDefault {
			return ErrQuotaExceeded
		}
		if s.limits.MaxFilesClient > 0 && clientCount >= s.limits.MaxFilesClient {
			return ErrFileLimit
		}
	}
	return nil
}

func (s *Store) loadManifest() error {
	data, err := os.ReadFile(s.manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: read manifest: %w", err)
	}
	var doc struct {
		Files map[string]*File `json:"files"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("store: parse manifest: %w", err)
	}
	for id, f := range doc.Files {
		if f == nil || f.ID != id || f.Name == "" {
			continue
		}
		s.files[id] = f
	}
	return nil
}

func (s *Store) purgeOrphans() {
	for id, f := range s.files {
		if f.DerivedFrom != "" {
			if _, ok := s.files[f.DerivedFrom]; !ok {
				_ = os.Remove(filepath.Join(s.blobs, id))
				delete(s.files, id)
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(s.blobs, id)); errors.Is(err, os.ErrNotExist) {
			delete(s.files, id)
		}
	}
}

func (s *Store) reclaimUnreferenced(now time.Time) {
	entries, err := os.ReadDir(s.blobs)
	if err != nil {
		return
	}

	blobs := 0
	for _, e := range entries {
		if !e.IsDir() {
			blobs++
		}
	}
	if len(s.files) == 0 && blobs > 0 {
		s.reclaimBlocked = true
		return
	}
	s.reclaimBlocked = false

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, ok := s.files[e.Name()]; ok {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < reclaimGrace {
			continue
		}
		_ = os.Remove(filepath.Join(s.blobs, e.Name()))
	}

	roots, err := os.ReadDir(s.root)
	if err != nil {
		return
	}
	for _, e := range roots {
		if e.IsDir() || !strings.HasPrefix(e.Name(), tempPrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil || now.Sub(info.ModTime()) < tempMaxAge {
			continue
		}
		_ = os.Remove(filepath.Join(s.root, e.Name()))
	}
}

func (s *Store) ReclaimBlocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reclaimBlocked
}

func (s *Store) persistManifest() error {
	doc := struct {
		Version int              `json:"version"`
		Files   map[string]*File `json:"files"`
	}{Version: manifestVer, Files: s.files}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("store: marshal manifest: %w", err)
	}

	tmp := s.manifestPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("store: open manifest: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("store: write manifest: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("store: sync manifest: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("store: close manifest: %w", err)
	}
	if err := os.Rename(tmp, s.manifestPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("store: commit manifest: %w", err)
	}
	return nil
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case 0, '\n', '\r', '\t', '"':
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		name = cleanName
	}
	if len(name) > maxNameLength {
		name = name[:maxNameLength]
	}
	return name
}

func newID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: random id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
