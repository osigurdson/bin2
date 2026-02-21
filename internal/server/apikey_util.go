package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"regexp"

	"bin2.io/internal/db"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type apiKeyCredential struct {
	UserID uuid.UUID
	Secret string
}

func generateAPIKeyCredential(userID uuid.UUID) (cred apiKeyCredential, hash string, err error) {
	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		return apiKeyCredential{}, "", err
	}

	base32Secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	cred = apiKeyCredential{
		UserID: userID,
		Secret: base32Secret,
	}

	credString := ConvertAPIKeyCredentialToString(cred)
	sha := sha256.Sum256([]byte(credString))
	hashBytes, err := bcrypt.GenerateFromPassword(sha[:], bcrypt.DefaultCost)
	if err != nil {
		return apiKeyCredential{}, "", err
	}

	return cred, string(hashBytes), nil
}

func ConvertAPIKeyCredentialToString(cred apiKeyCredential) string {
	return fmt.Sprintf("sk_%s_%s", cred.UserID, cred.Secret)
}

func parseAPIKeyCredential(apiKeyCredentialString string) (apiKeyCredential, error) {
	re := regexp.MustCompile(`^sk_([0-9a-fA-F-]{36})_(.+)$`)
	matches := re.FindStringSubmatch(apiKeyCredentialString)
	if len(matches) < 3 {
		return apiKeyCredential{}, fmt.Errorf("API Key credential string did not match expected format")
	}

	userID, err := uuid.Parse(matches[1])
	if err != nil {
		return apiKeyCredential{}, err
	}
	return apiKeyCredential{
		UserID: userID,
		Secret: matches[2],
	}, nil
}

func matchAPIKeyCredential(cred apiKeyCredential, keys []db.APIKey) (db.APIKey, error) {
	credString := ConvertAPIKeyCredentialToString(cred)
	secret := sha256.Sum256([]byte(credString))
	for _, key := range keys {
		if err := bcrypt.CompareHashAndPassword([]byte(key.SecretKeyHash), secret[:]); err == nil {
			return key, nil
		}
	}
	return db.APIKey{}, errUnauthorized
}
