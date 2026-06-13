package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_OK(t *testing.T) {
	p := writeTemp(t, `{
		"config": {"host": "127.0.0.1", "port": 8000},
		"gates": [{"name": "ethereum", "mnemonic": "test test test test test test test test test test test junk"}]
	}`)

	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr() != "127.0.0.1:8000" {
		t.Errorf("Addr() = %q", cfg.Addr())
	}
	if cfg.Mnemonics()["ethereum"] == "" {
		t.Error("expected mnemonic for ethereum gate")
	}
}

func TestLoad_Errors(t *testing.T) {
	cases := map[string]string{
		"no port":     `{"config":{"host":"x"},"gates":[{"name":"ethereum","mnemonic":"m"}]}`,
		"no gates":    `{"config":{"port":8000},"gates":[]}`,
		"empty name":  `{"config":{"port":8000},"gates":[{"name":"","mnemonic":"m"}]}`,
		"no mnemonic": `{"config":{"port":8000},"gates":[{"name":"ethereum"}]}`,
		"duplicate":   `{"config":{"port":8000},"gates":[{"name":"e","mnemonic":"m"},{"name":"e","mnemonic":"m"}]}`,
		"bad json":    `{`,
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeTemp(t, content)); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load("/no/such/file.json"); err == nil {
		t.Fatal("expected an error for missing file")
	}
}
