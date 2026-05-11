package config

import "testing"

func TestLoadExampleConfig(t *testing.T) {
	cfg, err := Load("../../configs/sites/example.yml")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Site != "example" {
		t.Fatalf("Site = %q, want example", cfg.Site)
	}
	if cfg.Agent.Mode != "passive-listening" {
		t.Fatalf("Agent.Mode = %q, want passive-listening", cfg.Agent.Mode)
	}
	if cfg.Agent.SubMode != "read-only" {
		t.Fatalf("Agent.SubMode = %q, want read-only", cfg.Agent.SubMode)
	}
	if len(cfg.NPorts) != 1 {
		t.Fatalf("len(NPorts) = %d, want 1", len(cfg.NPorts))
	}
	if len(cfg.Storage.Outputs) != 1 {
		t.Fatalf("len(Storage.Outputs) = %d, want 1", len(cfg.Storage.Outputs))
	}
}
