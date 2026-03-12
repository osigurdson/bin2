package main

import "testing"

func TestLoadSeedE2EConfigFromEnv(t *testing.T) {
	t.Setenv("BIN2_SEED_E2E_API_KEY", "sk_0123456789abcdef_ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")
	t.Setenv("BIN2_SEED_E2E_SUB", "seed:test-user")
	t.Setenv("BIN2_SEED_E2E_REGISTRY", "test")
	t.Setenv("BIN2_SEED_E2E_KEY_NAME", "")

	cfg, err := loadSeedE2EConfigFromEnv()
	if err != nil {
		t.Fatalf("loadSeedE2EConfigFromEnv: %v", err)
	}
	if cfg.KeyName != defaultSeedE2EKeyName {
		t.Fatalf("key name = %q, want %q", cfg.KeyName, defaultSeedE2EKeyName)
	}
}

func TestLoadSeedE2EConfigFromEnvRejectsInvalidKey(t *testing.T) {
	t.Setenv("BIN2_SEED_E2E_API_KEY", "not-a-key")
	t.Setenv("BIN2_SEED_E2E_SUB", "seed:test-user")
	t.Setenv("BIN2_SEED_E2E_REGISTRY", "test")

	if _, err := loadSeedE2EConfigFromEnv(); err == nil {
		t.Fatalf("expected invalid key to fail")
	}
}

func TestLoadSeedE2EConfigFromEnvRequiresRegistry(t *testing.T) {
	t.Setenv("BIN2_SEED_E2E_API_KEY", "sk_0123456789abcdef_ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")
	t.Setenv("BIN2_SEED_E2E_SUB", "seed:test-user")
	t.Setenv("BIN2_SEED_E2E_REGISTRY", "")

	if _, err := loadSeedE2EConfigFromEnv(); err == nil {
		t.Fatalf("expected missing registry to fail")
	}
}
