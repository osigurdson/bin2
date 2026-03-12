package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"bin2.io/internal/apikey"
	"bin2.io/internal/db"
	"github.com/google/uuid"
)

const defaultSeedE2EKeyName = "seed-e2e"
const maxSeedRegistryNameLen = 64

var seedRegistryNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type seedE2EConfig struct {
	APIKey   string
	Sub      string
	Registry string
	KeyName  string
}

func runSeedE2E(ctx context.Context) error {
	cfg, err := loadSeedE2EConfigFromEnv()
	if err != nil {
		return err
	}

	dbCfg, err := db.NewConfigFromEnv()
	if err != nil {
		return fmt.Errorf("could not read postgres configuration: %w", err)
	}

	conn, err := db.New(ctx, dbCfg)
	if err != nil {
		return fmt.Errorf("could not connect to postgres: %w", err)
	}
	defer conn.Close()

	apiKeyEncryptionKey, err := loadAPIKeyEncryptionKeyFromEnv()
	if err != nil {
		return err
	}

	registry, user, err := ensureSeedRegistryAndUser(ctx, conn, cfg)
	if err != nil {
		return err
	}

	if !user.Onboarded {
		if err := conn.SetUserOnboarded(ctx, user.ID, true); err != nil {
			return fmt.Errorf("could not mark seeded user onboarded: %w", err)
		}
	}

	if err := ensureSeedAPIKey(ctx, conn, user, registry, cfg, apiKeyEncryptionKey); err != nil {
		return err
	}

	log.Printf("seeded e2e user %q for registry %q with key name %q", cfg.Sub, cfg.Registry, cfg.KeyName)
	return nil
}

func loadSeedE2EConfigFromEnv() (seedE2EConfig, error) {
	cfg := seedE2EConfig{
		APIKey:   strings.TrimSpace(os.Getenv("BIN2_SEED_E2E_API_KEY")),
		Sub:      strings.TrimSpace(os.Getenv("BIN2_SEED_E2E_SUB")),
		Registry: strings.TrimSpace(os.Getenv("BIN2_SEED_E2E_REGISTRY")),
		KeyName:  strings.TrimSpace(os.Getenv("BIN2_SEED_E2E_KEY_NAME")),
	}
	if cfg.KeyName == "" {
		cfg.KeyName = defaultSeedE2EKeyName
	}

	if cfg.APIKey == "" {
		return seedE2EConfig{}, fmt.Errorf("BIN2_SEED_E2E_API_KEY is required")
	}
	if cfg.Sub == "" {
		return seedE2EConfig{}, fmt.Errorf("BIN2_SEED_E2E_SUB is required")
	}
	if cfg.Registry == "" {
		return seedE2EConfig{}, fmt.Errorf("BIN2_SEED_E2E_REGISTRY is required")
	}
	if len(cfg.Registry) > maxSeedRegistryNameLen || !seedRegistryNameRe.MatchString(cfg.Registry) {
		return seedE2EConfig{}, fmt.Errorf("BIN2_SEED_E2E_REGISTRY is invalid")
	}
	if _, err := apikey.ParsePrefix(cfg.APIKey); err != nil {
		return seedE2EConfig{}, fmt.Errorf("BIN2_SEED_E2E_API_KEY is invalid: %w", err)
	}

	return cfg, nil
}

func loadAPIKeyEncryptionKeyFromEnv() ([32]byte, error) {
	apiKeyEncKeyHex := strings.TrimSpace(os.Getenv("API_KEY_ENCRYPTION_KEY"))
	if apiKeyEncKeyHex == "" {
		return [32]byte{}, fmt.Errorf("API_KEY_ENCRYPTION_KEY is not defined")
	}
	apiKeyEncKeyBytes, err := hex.DecodeString(apiKeyEncKeyHex)
	if err != nil || len(apiKeyEncKeyBytes) != 32 {
		return [32]byte{}, fmt.Errorf("API_KEY_ENCRYPTION_KEY must be a 64-char hex string (32 bytes)")
	}
	var apiKeyEncryptionKey [32]byte
	copy(apiKeyEncryptionKey[:], apiKeyEncKeyBytes)
	return apiKeyEncryptionKey, nil
}

func ensureSeedRegistryAndUser(ctx context.Context, conn *db.DB, cfg seedE2EConfig) (db.Registry, db.User, error) {
	registry, err := conn.GetRegistryByName(ctx, cfg.Registry)
	switch {
	case err == nil:
		user, err := conn.GetOrCreateUserInTenant(ctx, cfg.Sub, registry.TenantID)
		if err != nil {
			return db.Registry{}, db.User{}, fmt.Errorf("could not create or reuse seeded user: %w", err)
		}
		return registry, user, nil
	case !errors.Is(err, db.ErrNotFound):
		return db.Registry{}, db.User{}, fmt.Errorf("could not resolve registry: %w", err)
	}

	user, err := conn.GetOrCreateUser(ctx, cfg.Sub, "")
	if err != nil {
		return db.Registry{}, db.User{}, fmt.Errorf("could not create or reuse seeded user: %w", err)
	}

	registry, err = conn.AddRegistry(ctx, db.AddRegistryArgs{
		OrgID: user.TenantID,
		Name:  cfg.Registry,
	})
	if errors.Is(err, db.ErrConflict) {
		registry, err = conn.GetRegistryByName(ctx, cfg.Registry)
	}
	if err != nil {
		return db.Registry{}, db.User{}, fmt.Errorf("could not create or reuse registry: %w", err)
	}

	user, err = conn.GetOrCreateUserInTenant(ctx, cfg.Sub, registry.TenantID)
	if err != nil {
		return db.Registry{}, db.User{}, fmt.Errorf("could not align seeded user with registry tenant: %w", err)
	}

	return registry, user, nil
}

func ensureSeedAPIKey(
	ctx context.Context,
	conn *db.DB,
	user db.User,
	registry db.Registry,
	cfg seedE2EConfig,
	encKey [32]byte,
) error {
	prefix, err := apikey.ParsePrefix(cfg.APIKey)
	if err != nil {
		return fmt.Errorf("could not parse seeded api key: %w", err)
	}

	existingByPrefix, err := conn.GetAPIKeyByPrefix(ctx, prefix)
	switch {
	case err == nil:
		decrypted, err := apikey.Decrypt(existingByPrefix.SecretEncrypted, encKey)
		if err != nil {
			return fmt.Errorf("could not decrypt existing api key with prefix %s: %w", prefix, err)
		}
		if !apikey.Match(cfg.APIKey, decrypted) {
			return fmt.Errorf("api key prefix %s already exists with different secret", prefix)
		}
		if existingByPrefix.UserID != user.ID {
			return fmt.Errorf("api key prefix %s already belongs to a different user", prefix)
		}
		scopes, err := conn.ListAPIKeyScopesByAPIKeyID(ctx, existingByPrefix.ID)
		if err != nil {
			return fmt.Errorf("could not inspect existing api key scopes: %w", err)
		}
		if existingByPrefix.KeyName == cfg.KeyName && apiKeyHasRegistryAdminScope(scopes, registry.ID) {
			return nil
		}
		if err := conn.RemoveAPIKey(ctx, user.ID, existingByPrefix.ID); err != nil {
			return fmt.Errorf("could not replace existing api key: %w", err)
		}
	case !errors.Is(err, db.ErrNotFound):
		return fmt.Errorf("could not resolve existing api key by prefix: %w", err)
	}

	keys, err := conn.ListAPIKeysByUser(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("could not list existing api keys: %w", err)
	}
	for _, key := range keys {
		if key.KeyName != cfg.KeyName || key.Prefix == prefix {
			continue
		}
		if err := conn.RemoveAPIKey(ctx, user.ID, key.ID); err != nil {
			return fmt.Errorf("could not remove replaced api key %s: %w", key.KeyName, err)
		}
	}

	encrypted, err := apikey.Encrypt(cfg.APIKey, encKey)
	if err != nil {
		return fmt.Errorf("could not encrypt seeded api key: %w", err)
	}

	if _, err := conn.AddAPIKey(ctx, db.AddAPIKeyArgs{
		UserID:          user.ID,
		KeyName:         cfg.KeyName,
		SecretEncrypted: encrypted,
		Prefix:          prefix,
		Scopes: []db.AddAPIKeyScopeInput{
			{
				RegistryID: registry.ID,
				Permission: db.APIKeyPermissionAdmin,
			},
		},
	}); err != nil {
		return fmt.Errorf("could not create seeded api key: %w", err)
	}

	return nil
}

func apiKeyHasRegistryAdminScope(scopes []db.APIKeyScope, registryID uuid.UUID) bool {
	for _, scope := range scopes {
		if scope.RegistryID != registryID {
			continue
		}
		if scope.RepositoryID != nil {
			continue
		}
		if scope.Permission == db.APIKeyPermissionAdmin {
			return true
		}
	}
	return false
}
