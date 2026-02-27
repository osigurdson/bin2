package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bin2.io/internal/db"
	"github.com/google/uuid"
)

func (s *Server) trackRegistryBlobDigest(ctx context.Context, digest string) error {
	if s.db == nil {
		return nil
	}
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return nil
	}
	return s.db.UpsertRegistryBlob(ctx, digest)
}

func (s *Server) indexRegistryManifest(
	ctx context.Context,
	registryID uuid.UUID,
	repo string,
	manifestDigest string,
	manifestBytes []byte,
	contentType string,
	references []string,
	blobDigests []string,
) error {
	if s.db == nil {
		return nil
	}

	args := db.UpsertRegistryManifestIndexArgs{
		RegistryID:     registryID,
		Repository:     strings.TrimSpace(repo),
		ManifestDigest: strings.TrimSpace(manifestDigest),
		ManifestBody:   manifestBytes,
		ContentType:    strings.TrimSpace(contentType),
		References:     references,
		BlobDigests:    blobDigests,
	}
	return s.db.UpsertRegistryManifestIndex(ctx, args)
}

func (s *Server) resolveRegistryIDForRepo(ctx context.Context, auth registryAuthContext, repo string) (uuid.UUID, error) {
	if auth.registryID != uuid.Nil {
		return auth.registryID, nil
	}

	namespace := registryNamespace(repo)
	if namespace == "" {
		return uuid.Nil, fmt.Errorf("invalid repository namespace")
	}

	registryRec, err := s.db.GetRegistryByName(ctx, namespace)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return uuid.Nil, errUnauthorized
		}
		return uuid.Nil, err
	}
	return registryRec.ID, nil
}
