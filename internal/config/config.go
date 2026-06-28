package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the top-level application configuration.
type Config struct {
	HTTPAddr string
	MySQL    MySQLConfig
	JWT      JWTConfig
}

// MySQLConfig holds MySQL connection parameters loaded from a .cnf file.
type MySQLConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	Socket   string
	Database string
}

// JWTConfig holds JWT signing configuration.
type JWTConfig struct {
	// Secret is the HMAC-SHA256 signing key.
	// Read from env NEHUANG_JWT_SECRET; defaults to a dev placeholder.
	Secret string
	// AdminTokenTTL is the validity window for admin-side JWTs. Default: 24h.
	AdminTokenTTL time.Duration
	// FarmerTokenTTL is the validity window for farmer (mini-program) JWTs. Default: 72h.
	FarmerTokenTTL time.Duration
}

// Load reads configuration from environment variables and the MySQL defaults file.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr: envDefault("NEHUANG_HTTP_ADDR", ":8080"),
		JWT: JWTConfig{
			Secret:         envDefault("NEHUANG_JWT_SECRET", "nehuang-dev-secret-change-me"),
			AdminTokenTTL:  24 * time.Hour,
			FarmerTokenTTL: 72 * time.Hour,
		},
	}

	credPath := envDefault("NEHUANG_MYSQL_DEFAULTS_FILE", filepath.Join("configs", "mysql", "nehuang_agent.cnf"))
	mysqlCfg, err := loadMySQLDefaultsFile(credPath)
	if err != nil {
		return Config{}, err
	}
	cfg.MySQL = mysqlCfg

	return cfg, nil
}

func loadMySQLDefaultsFile(path string) (MySQLConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return MySQLConfig{}, fmt.Errorf("open mysql defaults file %s: %w", path, err)
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	if err := scanner.Err(); err != nil {
		return MySQLConfig{}, fmt.Errorf("read mysql defaults file %s: %w", path, err)
	}

	port := 3306
	if values["port"] != "" {
		p, err := strconv.Atoi(values["port"])
		if err != nil {
			return MySQLConfig{}, fmt.Errorf("invalid mysql port %q: %w", values["port"], err)
		}
		port = p
	}

	cfg := MySQLConfig{
		User:     values["user"],
		Password: values["password"],
		Host:     envDefaultValue(values["host"], "127.0.0.1"),
		Port:     port,
		Socket:   values["socket"],
		Database: values["database"],
	}
	if cfg.User == "" || cfg.Database == "" {
		return MySQLConfig{}, fmt.Errorf("mysql defaults file %s must include user and database", path)
	}

	return cfg, nil
}

func envDefault(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func envDefaultValue(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return strings.TrimSpace(val)
}
