package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var envKeys = []string{
	"TMPDROP_ADDR", "TMPDROP_STORAGE_DIR", "TMPDROP_MAX_FILE_SIZE",
	"TMPDROP_MAX_STORAGE", "TMPDROP_DEFAULT_TTL", "TMPDROP_SWEEP_INTERVAL",
	"TMPDROP_SHUTDOWN_TIMEOUT", "TMPDROP_QUOTA_DEFAULT",
	"TMPDROP_MAX_FILES_PER_CLIENT", "TMPDROP_LOG_LEVEL", "TMPDROP_CONFIG",
	"CONVERTX_URL", "CONVERTX_EMAIL", "CONVERTX_PASSWORD", "CONVERTX_TIMEOUT",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range envKeys {
		if v, ok := os.LookupEnv(k); ok {
			k, v := k, v
			os.Unsetenv(k)
			t.Cleanup(func() { os.Setenv(k, v) })
		}
	}
}

func TestDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.StorageDir != "./data" {
		t.Errorf("StorageDir = %q, want ./data", cfg.StorageDir)
	}
	if cfg.MaxFileSize != 100<<20 {
		t.Errorf("MaxFileSize = %d", cfg.MaxFileSize)
	}
	if cfg.MaxStorage != 1<<30 {
		t.Errorf("MaxStorage = %d", cfg.MaxStorage)
	}
	if cfg.DefaultTTL != 24*time.Hour {
		t.Errorf("DefaultTTL = %s", cfg.DefaultTTL)
	}
	if cfg.ConvertX.Enabled() {
		t.Errorf("ConvertX should be disabled by default")
	}
}

func TestParseSize(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int64
		ok   bool
	}{
		{"0", 0, true},
		{"1024", 1024, true},
		{"512B", 512, true},
		{"1KB", 1024, true},
		{"2MB", 2 << 20, true},
		{"1.5GB", 1<<30 + 1<<29, true},
		{"10G", 10 << 30, true},
		{"1k", 1024, true},
		{"1M", 1 << 20, true},
		{"", 0, false},
		{"-5MB", 0, false},
		{"abc", 0, false},
	} {
		got, err := ParseSize(tc.in)
		if tc.ok && err != nil {
			t.Errorf("ParseSize(%q) error: %v", tc.in, err)
			continue
		}
		if !tc.ok && err == nil {
			t.Errorf("ParseSize(%q) should fail", tc.in)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseSize(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestEnvOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("TMPDROP_ADDR", "127.0.0.1:9999")
	t.Setenv("TMPDROP_MAX_FILE_SIZE", "5MB")
	t.Setenv("TMPDROP_DEFAULT_TTL", "2h")
	t.Setenv("TMPDROP_QUOTA_DEFAULT", "0")
	t.Setenv("TMPDROP_MAX_FILES_PER_CLIENT", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9999" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.MaxFileSize != 5<<20 {
		t.Errorf("MaxFileSize = %d", cfg.MaxFileSize)
	}
	if cfg.DefaultTTL != 2*time.Hour {
		t.Errorf("DefaultTTL = %s", cfg.DefaultTTL)
	}
	if cfg.QuotaDefault != 0 {
		t.Errorf("QuotaDefault = %d, want 0 (disabled)", cfg.QuotaDefault)
	}
	if cfg.MaxFilesClient != 0 {
		t.Errorf("MaxFilesClient = %d, want 0", cfg.MaxFilesClient)
	}
}

func TestFileOverrides(t *testing.T) {
	clearEnv(t)
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{
		"addr": ":7000",
		"storage_dir": "/tmp/shared",
		"max_file_size": "64MB",
		"default_ttl": "1h",
		"max_files_per_client": 10,
		"convertx": {
			"url": "http://convertx:3000",
			"email": "a@b.c",
			"password": "secret"
		}
	}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDROP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":7000" || cfg.StorageDir != "/tmp/shared" {
		t.Errorf("file values not applied: %+v", cfg)
	}
	if cfg.MaxFileSize != 64<<20 {
		t.Errorf("MaxFileSize = %d", cfg.MaxFileSize)
	}
	if cfg.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %s", cfg.DefaultTTL)
	}
	if cfg.MaxFilesClient != 10 {
		t.Errorf("MaxFilesClient = %d", cfg.MaxFilesClient)
	}
	if !cfg.ConvertX.Enabled() {
		t.Errorf("ConvertX should be enabled")
	}
}

func TestEnvBeatsFile(t *testing.T) {
	clearEnv(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"max_file_size": "1MB", "addr": ":7000"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDROP_CONFIG", path)
	t.Setenv("TMPDROP_MAX_FILE_SIZE", "3MB")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxFileSize != 3<<20 {
		t.Errorf("MaxFileSize = %d, env should win", cfg.MaxFileSize)
	}
	if cfg.Addr != ":7000" {
		t.Errorf("Addr = %q, file value lost", cfg.Addr)
	}
}

func TestConvertXNeedsCredentials(t *testing.T) {
	clearEnv(t)
	t.Setenv("CONVERTX_URL", "http://convertx:3000")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error when URL set without credentials")
	}
}

func TestInvalidEnvValue(t *testing.T) {
	clearEnv(t)
	t.Setenv("TMPDROP_DEFAULT_TTL", "not-a-duration")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error for invalid duration")
	}
}
