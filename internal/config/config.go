package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr            = ":8080"
	defaultStorageDir      = "./data"
	defaultMaxFileSize     = 100 << 20
	defaultMaxStorage      = 1 << 30
	defaultTTL             = 24 * time.Hour
	defaultSweepInterval   = time.Minute
	defaultShutdownTimeout = 10 * time.Second
	defaultQuotaDefault    = 250 << 20
	defaultMaxFilesClient  = 50
	defaultLogLevel        = "info"
	DefaultSourceURL       = "https://github.com/jesusg703/tmpdrop"
	defaultConvertTimeout  = 5 * time.Minute
)

type Config struct {
	Addr            string
	StorageDir      string
	MaxFileSize     int64
	MaxStorage      int64
	DefaultTTL      time.Duration
	SweepInterval   time.Duration
	ShutdownTimeout time.Duration
	QuotaDefault    int64
	MaxFilesClient  int
	LogLevel        string
	SourceURL       string
	TrustedProxies  []string
	ConvertX        ConvertX
}

type ConvertX struct {
	URL      string
	Email    string
	Password string
	Timeout  time.Duration
}

func (c ConvertX) Enabled() bool { return c.URL != "" }

func defaults() Config {
	return Config{
		Addr:            defaultAddr,
		StorageDir:      defaultStorageDir,
		MaxFileSize:     defaultMaxFileSize,
		MaxStorage:      defaultMaxStorage,
		DefaultTTL:      defaultTTL,
		SweepInterval:   defaultSweepInterval,
		ShutdownTimeout: defaultShutdownTimeout,
		QuotaDefault:    defaultQuotaDefault,
		MaxFilesClient:  defaultMaxFilesClient,
		LogLevel:        defaultLogLevel,
		SourceURL:       DefaultSourceURL,
		ConvertX: ConvertX{
			Timeout: defaultConvertTimeout,
		},
	}
}

type fileConfig struct {
	Addr            *string       `json:"addr"`
	StorageDir      *string       `json:"storage_dir"`
	MaxFileSize     *string       `json:"max_file_size"`
	MaxStorage      *string       `json:"max_storage"`
	DefaultTTL      *string       `json:"default_ttl"`
	SweepInterval   *string       `json:"sweep_interval"`
	ShutdownTimeout *string       `json:"shutdown_timeout"`
	QuotaDefault    *string       `json:"quota_default"`
	MaxFilesClient  *int          `json:"max_files_per_client"`
	LogLevel        *string       `json:"log_level"`
	SourceURL       *string       `json:"source_url"`
	TrustedProxies  *[]string     `json:"trusted_proxies"`
	ConvertX        *fileConvertX `json:"convertx"`
}

type fileConvertX struct {
	URL      *string `json:"url"`
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Timeout  *string `json:"timeout"`
}

func Load() (Config, error) {
	cfg := defaults()

	if path := os.Getenv("TMPDROP_CONFIG"); path != "" {
		if err := applyFile(&cfg, path); err != nil {
			return Config{}, err
		}
	} else if _, err := os.Stat("config.json"); err == nil {
		if err := applyFile(&cfg, "config.json"); err != nil {
			return Config{}, err
		}
	}

	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return fmt.Errorf("config: parse %s: %w", path, err)
	}

	if fc.Addr != nil {
		cfg.Addr = *fc.Addr
	}
	if fc.StorageDir != nil {
		cfg.StorageDir = *fc.StorageDir
	}
	if fc.MaxFileSize != nil {
		if cfg.MaxFileSize, err = ParseSize(*fc.MaxFileSize); err != nil {
			return fmt.Errorf("config: max_file_size: %w", err)
		}
	}
	if fc.MaxStorage != nil {
		if cfg.MaxStorage, err = ParseSize(*fc.MaxStorage); err != nil {
			return fmt.Errorf("config: max_storage: %w", err)
		}
	}
	if fc.DefaultTTL != nil {
		if cfg.DefaultTTL, err = time.ParseDuration(*fc.DefaultTTL); err != nil {
			return fmt.Errorf("config: default_ttl: %w", err)
		}
	}
	if fc.SweepInterval != nil {
		if cfg.SweepInterval, err = time.ParseDuration(*fc.SweepInterval); err != nil {
			return fmt.Errorf("config: sweep_interval: %w", err)
		}
	}
	if fc.ShutdownTimeout != nil {
		if cfg.ShutdownTimeout, err = time.ParseDuration(*fc.ShutdownTimeout); err != nil {
			return fmt.Errorf("config: shutdown_timeout: %w", err)
		}
	}
	if fc.QuotaDefault != nil {
		if cfg.QuotaDefault, err = ParseSize(*fc.QuotaDefault); err != nil {
			return fmt.Errorf("config: quota_default: %w", err)
		}
	}
	if fc.MaxFilesClient != nil {
		cfg.MaxFilesClient = *fc.MaxFilesClient
	}
	if fc.LogLevel != nil {
		cfg.LogLevel = *fc.LogLevel
	}
	if fc.SourceURL != nil {
		cfg.SourceURL = *fc.SourceURL
	}
	if fc.TrustedProxies != nil {
		cfg.TrustedProxies = *fc.TrustedProxies
	}
	if fc.ConvertX != nil {
		if fc.ConvertX.URL != nil {
			cfg.ConvertX.URL = *fc.ConvertX.URL
		}
		if fc.ConvertX.Email != nil {
			cfg.ConvertX.Email = *fc.ConvertX.Email
		}
		if fc.ConvertX.Password != nil {
			cfg.ConvertX.Password = *fc.ConvertX.Password
		}
		if fc.ConvertX.Timeout != nil {
			if cfg.ConvertX.Timeout, err = time.ParseDuration(*fc.ConvertX.Timeout); err != nil {
				return fmt.Errorf("config: convertx.timeout: %w", err)
			}
		}
	}
	return nil
}

func applyEnv(cfg *Config) error {
	var err error
	if cfg.Addr, err = envString(cfg.Addr, "TMPDROP_ADDR"); err != nil {
		return err
	}
	if cfg.StorageDir, err = envString(cfg.StorageDir, "TMPDROP_STORAGE_DIR"); err != nil {
		return err
	}
	if cfg.MaxFileSize, err = envSize(cfg.MaxFileSize, "TMPDROP_MAX_FILE_SIZE"); err != nil {
		return err
	}
	if cfg.MaxStorage, err = envSize(cfg.MaxStorage, "TMPDROP_MAX_STORAGE"); err != nil {
		return err
	}
	if cfg.DefaultTTL, err = envDuration(cfg.DefaultTTL, "TMPDROP_DEFAULT_TTL"); err != nil {
		return err
	}
	if cfg.SweepInterval, err = envDuration(cfg.SweepInterval, "TMPDROP_SWEEP_INTERVAL"); err != nil {
		return err
	}
	if cfg.ShutdownTimeout, err = envDuration(cfg.ShutdownTimeout, "TMPDROP_SHUTDOWN_TIMEOUT"); err != nil {
		return err
	}
	if cfg.QuotaDefault, err = envSize(cfg.QuotaDefault, "TMPDROP_QUOTA_DEFAULT"); err != nil {
		return err
	}
	if cfg.MaxFilesClient, err = envInt(cfg.MaxFilesClient, "TMPDROP_MAX_FILES_PER_CLIENT"); err != nil {
		return err
	}
	if cfg.LogLevel, err = envString(cfg.LogLevel, "TMPDROP_LOG_LEVEL"); err != nil {
		return err
	}
	if cfg.SourceURL, err = envString(cfg.SourceURL, "TMPDROP_SOURCE_URL"); err != nil {
		return err
	}
	cfg.TrustedProxies = envList(cfg.TrustedProxies, "TMPDROP_TRUSTED_PROXIES")
	if cfg.ConvertX.URL, err = envString(cfg.ConvertX.URL, "CONVERTX_URL"); err != nil {
		return err
	}
	if cfg.ConvertX.Email, err = envString(cfg.ConvertX.Email, "CONVERTX_EMAIL"); err != nil {
		return err
	}
	if cfg.ConvertX.Password, err = envString(cfg.ConvertX.Password, "CONVERTX_PASSWORD"); err != nil {
		return err
	}
	if cfg.ConvertX.Timeout, err = envDuration(cfg.ConvertX.Timeout, "CONVERTX_TIMEOUT"); err != nil {
		return err
	}
	return nil
}

func (c Config) validate() error {
	var bad []string
	if c.StorageDir == "" {
		bad = append(bad, "storage_dir")
	}
	if c.MaxFileSize <= 0 {
		bad = append(bad, "max_file_size")
	}
	if c.MaxStorage <= 0 {
		bad = append(bad, "max_storage")
	}
	if c.DefaultTTL < 0 {
		bad = append(bad, "default_ttl")
	}
	if c.SweepInterval <= 0 {
		bad = append(bad, "sweep_interval")
	}
	if c.ShutdownTimeout <= 0 {
		bad = append(bad, "shutdown_timeout")
	}
	if c.QuotaDefault < 0 {
		bad = append(bad, "quota_default")
	}
	if c.MaxFilesClient < 0 {
		bad = append(bad, "max_files_per_client")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		bad = append(bad, "log_level")
	}
	if len(bad) > 0 {
		return fmt.Errorf("config: invalid values for: %s", strings.Join(bad, ", "))
	}
	if c.ConvertX.URL != "" && (c.ConvertX.Email == "" || c.ConvertX.Password == "") {
		return errors.New("config: convertx.url requires convertx.email and convertx.password")
	}
	return nil
}

func ParseSize(s string) (int64, error) {
	raw := strings.TrimSpace(strings.ToUpper(s))
	if raw == "" {
		return 0, errors.New("config: empty size")
	}

	mult := int64(1)
	for _, sfx := range []struct {
		suf string
		mul int64
	}{
		{"KB", 1 << 10}, {"MB", 1 << 20}, {"GB", 1 << 30}, {"TB", 1 << 40},
		{"K", 1 << 10}, {"M", 1 << 20}, {"G", 1 << 30}, {"T", 1 << 40},
		{"B", 1},
	} {
		if strings.HasSuffix(raw, sfx.suf) {
			mult = sfx.mul
			raw = strings.TrimSuffix(raw, sfx.suf)
			break
		}
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("config: invalid size %q", s)
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("config: invalid size %q", s)
	}
	return int64(f * float64(mult)), nil
}

func envString(cur, name string) (string, error) {
	v, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(v) == "" {
		return cur, nil
	}
	return strings.TrimSpace(v), nil
}

func envSize(cur int64, name string) (int64, error) {
	v, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(v) == "" {
		return cur, nil
	}
	n, err := ParseSize(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s: %w", name, err)
	}
	return n, nil
}

func envDuration(cur time.Duration, name string) (time.Duration, error) {
	v, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(v) == "" {
		return cur, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s: %w", name, err)
	}
	return d, nil
}

func envInt(cur int, name string) (int, error) {
	v, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(v) == "" {
		return cur, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s: %w", name, err)
	}
	return n, nil
}

func envList(cur []string, name string) []string {
	v, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(v) == "" {
		return cur
	}
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
