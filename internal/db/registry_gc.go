package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

type RepositoryManifestRecord struct {
	Digest      string
	ContentType string
	Size        int64
	Body        []byte
}

func (d *DB) UpsertRegistryBlob(ctx context.Context, digest string, sizeBytes int64) error {
	if sizeBytes <= 0 {
		return fmt.Errorf("blob size must be > 0")
	}

	const cmd = `INSERT INTO blobs (digest, size_bytes)
		VALUES ($1, $2)
		ON CONFLICT (digest)
		DO UPDATE SET
			size_bytes = EXCLUDED.size_bytes,
			last_seen_at = NOW()`
	_, err := d.conn.Exec(ctx, cmd, strings.TrimSpace(digest), sizeBytes)
	return err
}

func (d *DB) GetRegistryBlobSize(ctx context.Context, digest string) (int64, error) {
	const cmd = `SELECT size_bytes FROM blobs WHERE digest = $1`

	var sizeBytes int64
	err := d.conn.QueryRow(ctx, cmd, strings.TrimSpace(digest)).Scan(&sizeBytes)
	if err != nil {
		if isNoRows(err) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return sizeBytes, nil
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

func (d *DB) BlobReferenced(ctx context.Context, digest string) (bool, error) {
	const cmd = `SELECT 1
		FROM manifest_blob_refs mb
		JOIN manifest_refs mr
		  ON mr.repository_id = mb.repository_id
		 AND mr.manifest_digest = mb.manifest_digest
		WHERE mb.blob_digest = $1
		LIMIT 1`

	var exists int
	err := d.conn.QueryRow(ctx, cmd, strings.TrimSpace(digest)).Scan(&exists)
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DB) DeleteManifestReference(
	ctx context.Context,
	registryID uuid.UUID,
	repository string,
	reference string,
) (bool, error) {
	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	repositoryID, err := lookupRepositoryID(ctx, tx, registryID, repository)
	if err != nil {
		if err == ErrNotFound {
			return false, nil
		}
		return false, err
	}

	const deleteRefCmd = `DELETE FROM manifest_refs
		WHERE repository_id = $1
		  AND reference = $2
		RETURNING manifest_digest`

	var manifestDigest string
	err = tx.QueryRow(
		ctx,
		deleteRefCmd,
		repositoryID,
		strings.TrimSpace(reference),
	).Scan(&manifestDigest)
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}

	if err := cleanupManifestArtifacts(ctx, tx, repositoryID, manifestDigest); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (d *DB) DeleteManifestByDigestInRepository(
	ctx context.Context,
	registryID uuid.UUID,
	repository string,
	manifestDigest string,
) (bool, error) {
	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	repositoryID, err := lookupRepositoryID(ctx, tx, registryID, repository)
	if err != nil {
		if err == ErrNotFound {
			return false, nil
		}
		return false, err
	}

	const deleteRefsCmd = `DELETE FROM manifest_refs
		WHERE repository_id = $1
		  AND manifest_digest = $2
		RETURNING manifest_digest`

	rows, err := tx.Query(
		ctx,
		deleteRefsCmd,
		repositoryID,
		strings.TrimSpace(manifestDigest),
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	deleted := false
	for rows.Next() {
		deleted = true
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if !deleted {
		return false, nil
	}

	if err := cleanupManifestArtifacts(ctx, tx, repositoryID, manifestDigest); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
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

func (d *DB) ListRepositoryManifestRecords(ctx context.Context, registryID uuid.UUID, repository string) ([]RepositoryManifestRecord, error) {
	const cmd = `SELECT DISTINCT ON (m.digest)
			m.digest,
			m.content_type,
			OCTET_LENGTH(m.body),
			m.body
		FROM repositories r
		JOIN manifest_refs mr ON mr.repository_id = r.id
		JOIN manifests m ON m.digest = mr.manifest_digest
		WHERE r.registry_id = $1
		  AND r.name = $2
		ORDER BY m.digest ASC`

	rows, err := d.conn.Query(
		ctx,
		cmd,
		registryID,
		strings.TrimSpace(repository),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]RepositoryManifestRecord, 0)
	for rows.Next() {
		var record RepositoryManifestRecord
		if err := rows.Scan(
			&record.Digest,
			&record.ContentType,
			&record.Size,
			&record.Body,
		); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (d *DB) HasManifestDigestInRepository(ctx context.Context, registryID uuid.UUID, repository, manifestDigest string) (bool, error) {
	const cmd = `SELECT 1
		FROM repositories r
		JOIN manifest_refs mr ON mr.repository_id = r.id
		WHERE r.registry_id = $1
		  AND r.name = $2
		  AND mr.manifest_digest = $3
		LIMIT 1`

	var exists int
	err := d.conn.QueryRow(
		ctx,
		cmd,
		registryID,
		strings.TrimSpace(repository),
		strings.TrimSpace(manifestDigest),
	).Scan(&exists)
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DB) ListRepositoryTags(ctx context.Context, registryID uuid.UUID, repository string, limit int, last string) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}

	const cmd = `SELECT mr.reference
		FROM repositories r
		JOIN manifest_refs mr ON mr.repository_id = r.id
		WHERE r.registry_id = $1
		  AND r.name = $2
		  AND mr.reference !~ '^sha256:[a-f0-9]{64}$'
		  AND (
		    $3 = ''
		    OR LOWER(mr.reference) > LOWER($3)
		    OR (LOWER(mr.reference) = LOWER($3) AND mr.reference > $3)
		  )
		ORDER BY LOWER(mr.reference) ASC, mr.reference ASC
		LIMIT $4`

	rows, err := d.conn.Query(
		ctx,
		cmd,
		registryID,
		strings.TrimSpace(repository),
		strings.TrimSpace(last),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]string, 0, limit)
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (d *DB) GetRegistryReferencedBlobBytes(ctx context.Context, registryID uuid.UUID) (int64, error) {
	const cmd = `SELECT COALESCE(SUM(b.size_bytes), 0)
		FROM (
			SELECT DISTINCT mb.blob_digest
			FROM repositories r
			JOIN manifest_refs mr
			  ON mr.repository_id = r.id
			JOIN manifest_blob_refs mb
			  ON mb.repository_id = mr.repository_id
			 AND mb.manifest_digest = mr.manifest_digest
			WHERE r.registry_id = $1
		) referenced
		JOIN blobs b
		  ON b.digest = referenced.blob_digest`

	var total int64
	if err := d.conn.QueryRow(ctx, cmd, registryID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
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

func lookupRepositoryID(ctx context.Context, tx pgx.Tx, registryID uuid.UUID, repository string) (uuid.UUID, error) {
	const cmd = `SELECT id
		FROM repositories
		WHERE registry_id = $1
		  AND name = $2
		LIMIT 1`

	var repositoryID uuid.UUID
	err := tx.QueryRow(ctx, cmd, registryID, strings.TrimSpace(repository)).Scan(&repositoryID)
	if err != nil {
		if isNoRows(err) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}
	return repositoryID, nil
}

func cleanupManifestArtifacts(ctx context.Context, tx pgx.Tx, repositoryID uuid.UUID, manifestDigest string) error {
	manifestDigest = strings.TrimSpace(manifestDigest)
	if manifestDigest == "" {
		return nil
	}

	const deleteBlobRefsCmd = `DELETE FROM manifest_blob_refs
		WHERE repository_id = $1
		  AND manifest_digest = $2
		  AND NOT EXISTS (
		    SELECT 1
		    FROM manifest_refs
		    WHERE repository_id = $1
		      AND manifest_digest = $2
		  )`
	if _, err := tx.Exec(ctx, deleteBlobRefsCmd, repositoryID, manifestDigest); err != nil {
		return err
	}

	const deleteManifestCmd = `DELETE FROM manifests
		WHERE digest = $1
		  AND NOT EXISTS (
		    SELECT 1
		    FROM manifest_refs
		    WHERE manifest_digest = $1
		  )`
	_, err := tx.Exec(ctx, deleteManifestCmd, manifestDigest)
	return err
}
