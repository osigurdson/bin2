package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type UpsertRegistryManifestIndexArgs struct {
	RegistryID     uuid.UUID
	Repository     string
	ManifestDigest string
	ManifestBody   []byte
	ContentType    string
	References     []string
	BlobDigests    []string
}

func (d *DB) UpsertRegistryBlob(ctx context.Context, digest string) error {
	const cmd = `INSERT INTO blobs (digest)
		VALUES ($1)
		ON CONFLICT (digest)
		DO UPDATE SET last_seen_at = NOW()`
	_, err := d.conn.Exec(ctx, cmd, strings.TrimSpace(digest))
	return err
}

func (d *DB) UpsertRegistryManifestIndex(ctx context.Context, args UpsertRegistryManifestIndexArgs) error {
	repository := strings.TrimSpace(args.Repository)
	manifestDigest := strings.TrimSpace(args.ManifestDigest)
	contentType := strings.TrimSpace(args.ContentType)
	if repository == "" {
		return fmt.Errorf("repository is required")
	}
	if manifestDigest == "" {
		return fmt.Errorf("manifest digest is required")
	}
	if len(args.ManifestBody) == 0 {
		return fmt.Errorf("manifest body is required")
	}
	if contentType == "" {
		return fmt.Errorf("manifest content type is required")
	}

	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	repositoryID := uuid.New()
	const upsertRepositoryCmd = `INSERT INTO repositories (id, registry_id, name, last_pushed_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (registry_id, name)
		DO UPDATE SET last_pushed_at = NOW()
		RETURNING id`
	if err := tx.QueryRow(ctx, upsertRepositoryCmd, repositoryID, args.RegistryID, repository).Scan(&repositoryID); err != nil {
		return err
	}

	const upsertManifestCmd = `INSERT INTO manifests (digest, content_type, body)
		VALUES ($1, $2, $3)
		ON CONFLICT (digest)
		DO UPDATE SET
			content_type = EXCLUDED.content_type,
			body = EXCLUDED.body`
	if _, err := tx.Exec(ctx, upsertManifestCmd, manifestDigest, contentType, args.ManifestBody); err != nil {
		return err
	}

	const insertBlobRefCmd = `INSERT INTO manifest_blob_refs (
		repository_id, manifest_digest, blob_digest
	) VALUES ($1, $2, $3)
	ON CONFLICT DO NOTHING`

	for _, blobDigest := range dedupeNonEmpty(args.BlobDigests) {
		if _, err := tx.Exec(
			ctx,
			insertBlobRefCmd,
			repositoryID,
			manifestDigest,
			blobDigest,
		); err != nil {
			return err
		}
	}

	const upsertManifestRefCmd = `INSERT INTO manifest_refs (
		repository_id, reference, manifest_digest
	) VALUES ($1, $2, $3)
	ON CONFLICT (repository_id, reference)
	DO UPDATE SET
		manifest_digest = EXCLUDED.manifest_digest,
		updated_at = NOW()`

	for _, reference := range dedupeNonEmpty(args.References) {
		if _, err := tx.Exec(
			ctx,
			upsertManifestRefCmd,
			repositoryID,
			reference,
			manifestDigest,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (d *DB) ListUnreferencedRegistryBlobDigests(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}

	const cmd = `WITH referenced_blobs AS (
		SELECT DISTINCT mb.blob_digest
		FROM manifest_blob_refs mb
		JOIN manifest_refs mr
		  ON mr.repository_id = mb.repository_id
		 AND mr.manifest_digest = mb.manifest_digest
	)
	SELECT b.digest
	FROM blobs b
	LEFT JOIN referenced_blobs r
	  ON r.blob_digest = b.digest
	WHERE r.blob_digest IS NULL
	ORDER BY b.last_seen_at ASC
	LIMIT $1`

	rows, err := d.conn.Query(ctx, cmd, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	digests := make([]string, 0, limit)
	for rows.Next() {
		var digest string
		if err := rows.Scan(&digest); err != nil {
			return nil, err
		}
		digests = append(digests, digest)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return digests, nil
}

func (d *DB) DeleteRegistryBlob(ctx context.Context, digest string) error {
	const cmd = `DELETE FROM blobs WHERE digest = $1`
	_, err := d.conn.Exec(ctx, cmd, strings.TrimSpace(digest))
	return err
}

func (d *DB) GetManifestByReference(ctx context.Context, registryID uuid.UUID, repository, reference string) ([]byte, string, string, error) {
	const cmd = `SELECT m.body, m.content_type, m.digest
		FROM repositories r
		JOIN manifest_refs mr ON mr.repository_id = r.id
		JOIN manifests m ON m.digest = mr.manifest_digest
		WHERE r.registry_id = $1
		  AND r.name = $2
		  AND mr.reference = $3
		LIMIT 1`

	var body []byte
	var contentType string
	var digest string
	err := d.conn.QueryRow(
		ctx,
		cmd,
		registryID,
		strings.TrimSpace(repository),
		strings.TrimSpace(reference),
	).Scan(&body, &contentType, &digest)
	if err != nil {
		if isNoRows(err) {
			return nil, "", "", ErrNotFound
		}
		return nil, "", "", err
	}
	return body, contentType, digest, nil
}

func dedupeNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
