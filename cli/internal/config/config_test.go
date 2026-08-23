package config

import (
	"path/filepath"
	"testing"

	"github.com/velgardey/yok/cli/internal/types"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	prev := configFile
	t.Cleanup(func() { configFile = prev })
	configFile = filepath.Join(dir, "cfg.json")

	in := types.Config{ProjectID: "p1", APIURL: "https://api.example.dev", Token: "yok_abc", SiteDomain: "dev.example"}
	if err := SaveConfig(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out != in {
		t.Fatalf("round trip mismatch: %+v != %+v", out, in)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	prev := configFile
	t.Cleanup(func() { configFile = prev })
	configFile = filepath.Join(t.TempDir(), "missing.json")
	out, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out.ProjectID != "" {
		t.Fatalf("expected empty config, got %+v", out)
	}
}
