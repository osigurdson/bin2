package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"

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
