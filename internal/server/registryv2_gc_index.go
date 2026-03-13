package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bin2.io/internal/db"
	"github.com/google/uuid"
)

// resolveTenantID returns the registryID and tenantID for a repository path.
// It uses the auth context if registryID is already known, otherwise resolves
// by registry name.
func (s *Server) resolveTenantID(ctx context.Context, auth registryAuthContext, repo string) (registryID, tenantID uuid.UUID, err error) {
	registryID, err = s.resolveRegistryIDForRepo(ctx, auth, repo)
	if err != nil {
		return
	}
	tenantID, err = s.db.GetRegistryTenantID(ctx, registryID)
	return
}

// emitUsageEvent inserts a single usage event, logging but not propagating any error.
func (s *Server) emitUsageEvent(ctx context.Context, tenantID, registryID uuid.UUID, repoID *uuid.UUID, digest, metric string, value int64) {
	if s.db == nil || tenantID == uuid.Nil {
		return
	}

	normalizedDigest, err := normalizeUsageEventDigest(digest)
	if err != nil {
		logError(fmt.Errorf("emitUsageEvent %s invalid digest %q: %w", metric, digest, err))
	}

	event := db.UsageEvent{
		ID:       uuid.New(),
		TenantID: tenantID,
		Digest:   normalizedDigest,
		Metric:   metric,
		Value:    value,
	}
	if registryID != uuid.Nil {
		event.RegistryID = &registryID
	}
	event.RepoID = repoID
	if err := s.db.InsertUsageEvents(ctx, []db.UsageEvent{event}); err != nil {
		logError(fmt.Errorf("emitUsageEvent %s: %w", metric, err))
	}
}

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
