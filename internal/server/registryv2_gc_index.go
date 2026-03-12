package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bin2.io/internal/db"
	"github.com/google/uuid"
)

func (s *Server) trackRegistryBlobDigest(ctx context.Context, digest string, sizeBytes int64) error {
	if s.db == nil {
		return nil
	}
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return nil
	}
	return s.db.UpsertObjectBlob(ctx, digest, sizeBytes)
}

func (s *Server) trackedRegistryBlobSize(ctx context.Context, digest string) (int64, bool, error) {
	if s.db == nil {
		return 0, false, nil
	}
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return 0, false, nil
	}
	size, err := s.db.GetObjectSize(ctx, digest)
	if errors.Is(err, db.ErrNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return size, true, nil
}

func (s *Server) noteObjectExistenceCheck(ctx context.Context, digest string) {
	if s.db == nil {
		return
	}
	if !s.probeCache.shouldUpdate(digest) {
		return
	}
	if err := s.db.NoteObjectExistenceCheck(ctx, digest); err != nil {
		logError(fmt.Errorf("could not note existence check for %s: %w", digest, err))
	}
}

func (s *Server) indexRegistryManifest(
	ctx context.Context,
	registryID uuid.UUID,
	repo string,
	manifestDigest string,
	manifestBytes []byte,
	contentType string,
	tag string,
	blobDigests []string,
	childManifestDigests []string,
	subjectDigest string,
) error {
	if s.db == nil {
		return nil
	}

	objectType := "manifest"
	if len(childManifestDigests) > 0 {
		objectType = "manifest_index"
	}

	return s.db.UpsertManifest(ctx, db.UpsertManifestArgs{
		RegistryID:           registryID,
		Repository:           strings.TrimSpace(repo),
		ManifestDigest:       strings.TrimSpace(manifestDigest),
		ManifestBody:         manifestBytes,
		ContentType:          strings.TrimSpace(contentType),
		ObjectType:           objectType,
		Tag:                  strings.TrimSpace(tag),
		BlobDigests:          blobDigests,
		ChildManifestDigests: childManifestDigests,
		SubjectDigest:        strings.TrimSpace(subjectDigest),
	})
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
