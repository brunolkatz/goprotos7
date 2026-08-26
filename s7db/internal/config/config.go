package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type PLCConfig struct {
	Addr string `yaml:"addr"`
	Rack *int   `yaml:"rack"`
	Slot *int   `yaml:"slot"`
	Port *int   `yaml:"port"`
}

type Config struct {
	File    string    `yaml:"file"`
	DB      *int      `yaml:"db"`
	Endian  string    `yaml:"endian"`
	Timeout string    `yaml:"timeout"`
	PLC     PLCConfig `yaml:"plc"`
}

func DefaultPath() string {
	if p := os.Getenv("S7DB_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "s7db", "config.yaml")
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	if path == "" {
		return Config{}, nil
	}

	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return Config{}, err
		}
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return Config{}, nil
			}
			return Config{}, err
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.Endian = strings.ToLower(strings.TrimSpace(cfg.Endian))
	return cfg, nil
}

func (c Config) TimeoutDuration() (time.Duration, error) {
	if strings.TrimSpace(c.Timeout) == "" {
		return 0, nil
	}
	return time.ParseDuration(c.Timeout)
}
