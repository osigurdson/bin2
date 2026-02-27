package server

import (
	"context"
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
		References:     references,
		BlobDigests:    blobDigests,
	}
	return s.db.UpsertRegistryManifestIndex(ctx, args)
}
