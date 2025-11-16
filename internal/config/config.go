package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Load загружает конфиг из TOML-файла.
// конфиг по приоритетам
// путь, явно переданный аргументом path (флаг -config),
// ENV CONFIG_PATH,
// дефолтный "configs/config.toml".
func Load(path string) (Config, error) {
	var cfg Config

	if path == "" {
		if env := os.Getenv("CONFIG_PATH"); env != "" {
			path = env
		} else {
			path = "configs/config.toml"
		}
	}

	// Делаем путь абсолютным, чтобы в логах было понятнее
	if !filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err == nil {
			path = filepath.Join(wd, path)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal toml from %q: %w", path, err)
	}

	return cfg, nil
}

// DSN для pgxpool.
func (p PGStorageConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		p.User, p.Password, p.Host, p.Port, p.DBName,
	)
}
