package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config повторяет структуру config.json.
type Config struct {
	Server Server `json:"config"`
	Gates  []Gate `json:"gates"`
}

type Server struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Gate struct {
	Name     string `json:"name"`
	Mnemonic string `json:"mnemonic"`
}

// Load читает и валидирует файл конфигурации.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Server.Port == 0 {
		return fmt.Errorf("config.port is required")
	}
	if len(c.Gates) == 0 {
		return fmt.Errorf("at least one gate is required")
	}

	seen := make(map[string]struct{}, len(c.Gates))
	for i, g := range c.Gates {
		switch {
		case g.Name == "":
			return fmt.Errorf("gates[%d].name is required", i)
		case g.Mnemonic == "":
			return fmt.Errorf("gate %q: mnemonic is required", g.Name)
		}
		if _, dup := seen[g.Name]; dup {
			return fmt.Errorf("duplicate gate %q", g.Name)
		}
		seen[g.Name] = struct{}{}
	}

	return nil
}

// Addr возвращает host:port для http.Server.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Mnemonics отдаёт мапу gate -> mnemonic для signer'а.
func (c *Config) Mnemonics() map[string]string {
	out := make(map[string]string, len(c.Gates))
	for _, g := range c.Gates {
		out[g.Name] = g.Mnemonic
	}
	return out
}
