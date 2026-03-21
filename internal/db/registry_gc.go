package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UpsertManifestArgs struct {
	RegistryID           uuid.UUID
	Repository           string
	ManifestDigest       string
	ManifestBody         []byte
	ContentType          string
	ObjectType           string   // "manifest" or "manifest_index"
	Tag                  string   // tag name; empty for digest-only push
	BlobDigests          []string // blob children (config + layers)
	ChildManifestDigests []string // manifest children (for manifest_index)
	SubjectDigest        string   // optional subject
}

type RepositoryManifestRecord struct {
	Digest      string
	ContentType string
	Size        int64
	Body        []byte
}

func (d *DB) UpsertObjectBlob(ctx context.Context, digest string, sizeBytes int64) error {
	if sizeBytes < 0 {
		return fmt.Errorf("blob size must be >= 0")
	}
	const cmd = `INSERT INTO objects (digest, size_bytes, type, content_type, storage)
		VALUES ($1, $2, 'blob', '', 'r2')
		ON CONFLICT (digest) DO UPDATE SET
			size_bytes = EXCLUDED.size_bytes,
			existence_checked_at = NOW()`
	_, err := d.conn.Exec(ctx, cmd, strings.TrimSpace(digest), sizeBytes)
	return err
}

func (d *DB) GetObjectSize(ctx context.Context, digest string) (int64, error) {
	const cmd = `SELECT size_bytes FROM objects WHERE digest = $1`
	var size int64
	err := d.conn.QueryRow(ctx, cmd, strings.TrimSpace(digest)).Scan(&size)
	if err != nil {
		if isNoRows(err) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return size, nil
}

func (d *DB) GetRepositoryObjectSize(ctx context.Context, registryID uuid.UUID, repository, digest string) (int64, error) {
	const cmd = `SELECT o.size_bytes
		FROM repositories r
		JOIN repository_objects ro ON ro.repository_id = r.id
		JOIN objects o ON o.digest = ro.digest
		WHERE r.registry_id = $1 AND r.name = $2 AND ro.digest = $3
		LIMIT 1`
	var size int64
	err := d.conn.QueryRow(ctx, cmd, registryID, strings.TrimSpace(repository), strings.TrimSpace(digest)).Scan(&size)
	if err != nil {
		if isNoRows(err) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return size, nil
}

func (d *DB) NoteObjectExistenceCheck(ctx context.Context, digest string) error {
	const cmd = `UPDATE objects SET existence_checked_at = NOW() WHERE digest = $1`
	_, err := d.conn.Exec(ctx, cmd, strings.TrimSpace(digest))
	return err
}

func (d *DB) UpsertManifest(ctx context.Context, args UpsertManifestArgs) error {
	repository := strings.TrimSpace(args.Repository)
	manifestDigest := strings.TrimSpace(args.ManifestDigest)
	contentType := strings.TrimSpace(args.ContentType)
	objectType := strings.TrimSpace(args.ObjectType)
	tag := strings.TrimSpace(args.Tag)

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
	if objectType == "" {
		objectType = "manifest"
	}

	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	repositoryID := uuid.New()
	const upsertRepoCmd = `INSERT INTO repositories (id, registry_id, name, last_pushed_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (registry_id, name)
		DO UPDATE SET last_pushed_at = NOW()
		RETURNING id`
	if err := tx.QueryRow(ctx, upsertRepoCmd, repositoryID, args.RegistryID, repository).Scan(&repositoryID); err != nil {
		return err
	}

	const upsertObjectCmd = `INSERT INTO objects (digest, size_bytes, type, content_type, storage, body)
		VALUES ($1, $2, $3, $4, 'db', $5)
		ON CONFLICT (digest) DO UPDATE SET
			content_type = EXCLUDED.content_type,
			body = EXCLUDED.body`
	if _, err := tx.Exec(ctx, upsertObjectCmd,
		manifestDigest, int64(len(args.ManifestBody)), objectType, contentType, args.ManifestBody,
	); err != nil {
		return err
	}

	const upsertRepoObjCmd = `INSERT INTO repository_objects (repository_id, digest)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`
	if _, err := tx.Exec(ctx, upsertRepoObjCmd, repositoryID, manifestDigest); err != nil {
		return err
	}
	for _, blobDigest := range dedupeNonEmpty(args.BlobDigests) {
		if _, err := tx.Exec(ctx, upsertRepoObjCmd, repositoryID, blobDigest); err != nil {
			return err
		}
	}

	if tag != "" {
		const upsertTagCmd = `INSERT INTO tags (repository_id, name, digest, updated_at)
			VALUES ($1, $2, $3, NOW())
			ON CONFLICT (repository_id, name)
			DO UPDATE SET digest = EXCLUDED.digest, updated_at = NOW()`
		if _, err := tx.Exec(ctx, upsertTagCmd, repositoryID, tag, manifestDigest); err != nil {
			return err
		}
	}

	const insertEdgeCmd = `INSERT INTO graph (parent_digest, child_digest, position, is_subject)
		VALUES ($1, $2, $3, false)
		ON CONFLICT DO NOTHING`
	for i, blobDigest := range dedupeNonEmpty(args.BlobDigests) {
		if _, err := tx.Exec(ctx, insertEdgeCmd, manifestDigest, blobDigest, i); err != nil {
			return err
		}
	}
	for i, childDigest := range dedupeNonEmpty(args.ChildManifestDigests) {
		if _, err := tx.Exec(ctx, insertEdgeCmd, manifestDigest, childDigest, i); err != nil {
			return err
		}
	}

	if subjectDigest := strings.TrimSpace(args.SubjectDigest); subjectDigest != "" {
		// Only insert the subject edge if the subject object exists; the OCI spec
		// requires registries to accept referrers whose subject has not been pushed.
		const insertSubjectCmd = `INSERT INTO graph (parent_digest, child_digest, position, is_subject)
			SELECT $1, $2, 0, true
			WHERE EXISTS (SELECT 1 FROM objects WHERE digest = $2)
			ON CONFLICT (parent_digest, child_digest)
			DO UPDATE SET is_subject = true`
		if _, err := tx.Exec(ctx, insertSubjectCmd, manifestDigest, subjectDigest); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (d *DB) GetManifestByReference(ctx context.Context, registryID uuid.UUID, repository, reference string) ([]byte, string, string, error) {
	repository = strings.TrimSpace(repository)
	reference = strings.TrimSpace(reference)

	if strings.HasPrefix(reference, "sha256:") {
		const cmd = `SELECT o.body, o.content_type, o.digest
			FROM repositories r
			JOIN repository_objects ro ON ro.repository_id = r.id
			JOIN objects o ON o.digest = ro.digest
			WHERE r.registry_id = $1 AND r.name = $2 AND ro.digest = $3
			LIMIT 1`
		var body []byte
		var contentType, digest string
		err := d.conn.QueryRow(ctx, cmd, registryID, repository, reference).Scan(&body, &contentType, &digest)
		if err != nil {
			if isNoRows(err) {
				return nil, "", "", ErrNotFound
			}
			return nil, "", "", err
		}
		return body, contentType, digest, nil
	}

	const cmd = `SELECT o.body, o.content_type, o.digest
		FROM repositories r
		JOIN tags t ON t.repository_id = r.id
		JOIN objects o ON o.digest = t.digest
		WHERE r.registry_id = $1 AND r.name = $2 AND t.name = $3
		LIMIT 1`
	var body []byte
	var contentType, digest string
	err := d.conn.QueryRow(ctx, cmd, registryID, repository, reference).Scan(&body, &contentType, &digest)
	if err != nil {
		if isNoRows(err) {
			return nil, "", "", ErrNotFound
		}
		return nil, "", "", err
	}
	return body, contentType, digest, nil
}

func (d *DB) HasManifestDigestInRepository(ctx context.Context, registryID uuid.UUID, repository, manifestDigest string) (bool, error) {
	const cmd = `SELECT 1
		FROM repositories r
		JOIN repository_objects ro ON ro.repository_id = r.id
		WHERE r.registry_id = $1 AND r.name = $2 AND ro.digest = $3
		LIMIT 1`
	var exists int
	err := d.conn.QueryRow(ctx, cmd, registryID, strings.TrimSpace(repository), strings.TrimSpace(manifestDigest)).Scan(&exists)
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DB) ListRepositoryManifestRecords(ctx context.Context, registryID uuid.UUID, repository string) ([]RepositoryManifestRecord, error) {
	const cmd = `SELECT DISTINCT ON (o.digest)
			o.digest,
			o.content_type,
			OCTET_LENGTH(o.body),
			o.body
		FROM repositories r
		JOIN repository_objects ro ON ro.repository_id = r.id
		JOIN objects o ON o.digest = ro.digest
		WHERE r.registry_id = $1
		  AND r.name = $2
		  AND o.type IN ('manifest', 'manifest_index')
		ORDER BY o.digest ASC`
	rows, err := d.conn.Query(ctx, cmd, registryID, strings.TrimSpace(repository))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]RepositoryManifestRecord, 0)
	for rows.Next() {
		var record RepositoryManifestRecord
		if err := rows.Scan(&record.Digest, &record.ContentType, &record.Size, &record.Body); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (d *DB) ListRepositoryTags(ctx context.Context, registryID uuid.UUID, repository string, limit int, last string) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	const cmd = `SELECT t.name
		FROM repositories r
		JOIN tags t ON t.repository_id = r.id
		WHERE r.registry_id = $1
		  AND r.name = $2
		  AND (
		    $3 = ''
		    OR LOWER(t.name) > LOWER($3)
		    OR (LOWER(t.name) = LOWER($3) AND t.name > $3)
		  )
		ORDER BY LOWER(t.name) ASC, t.name ASC
		LIMIT $4`
	rows, err := d.conn.Query(ctx, cmd, registryID, strings.TrimSpace(repository), strings.TrimSpace(last), limit)
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
	return tags, rows.Err()
}

func (d *DB) DeleteManifestReference(ctx context.Context, registryID uuid.UUID, repository, reference string) (bool, error) {
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

	const deleteTagCmd = `DELETE FROM tags WHERE repository_id = $1 AND name = $2`
	tag, err := tx.Exec(ctx, deleteTagCmd, repositoryID, strings.TrimSpace(reference))
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	return true, tx.Commit(ctx)
}

type DeletedBlobInfo struct {
	Digest    string
	SizeBytes int64
}

func (d *DB) DeleteManifestByDigestInRepository(
	ctx context.Context,
	registryID uuid.UUID,
	tenantID uuid.UUID,
	repository string,
	manifestDigest string,
) (deleted bool, orphanedBlobs []DeletedBlobInfo, err error) {
	tx, err := d.conn.Begin(ctx)
	if err != nil {
		return false, nil, err
	}
	defer tx.Rollback(ctx)

	repositoryID, err := lookupRepositoryID(ctx, tx, registryID, repository)
	if err != nil {
		if err == ErrNotFound {
			return false, nil, nil
		}
		return false, nil, err
	}

	manifestDigest = strings.TrimSpace(manifestDigest)

	// Guard: refuse to delete a manifest that is a child of another manifest
	// still present in this repository (e.g. a platform manifest inside an index).
	// The caller must delete the parent index first.
	// We deliberately exclude is_subject edges: those are OCI referrer relationships
	// (an artifact pointing AT a subject) and do NOT block deletion of the subject.
	const checkParentCmd = `SELECT EXISTS (
		SELECT 1 FROM graph g
		JOIN repository_objects ro ON ro.digest = g.parent_digest
		WHERE g.child_digest = $1 AND ro.repository_id = $2
		  AND g.is_subject = false
	)`
	var hasParent bool
	if err := tx.QueryRow(ctx, checkParentCmd, manifestDigest, repositoryID).Scan(&hasParent); err != nil {
		return false, nil, err
	}
	if hasParent {
		return false, nil, ErrManifestHasParent
	}

	// Verify the manifest is actually present in this repository.
	const checkPresentCmd = `SELECT 1 FROM repository_objects WHERE repository_id = $1 AND digest = $2`
	var dummy int
	if err := tx.QueryRow(ctx, checkPresentCmd, repositoryID, manifestDigest).Scan(&dummy); err != nil {
		if isNoRows(err) {
			return false, nil, nil
		}
		return false, nil, err
	}

	// Remove all tags pointing to this manifest digest.
	const deleteTagsCmd = `DELETE FROM tags WHERE repository_id = $1 AND digest = $2`
	if _, err := tx.Exec(ctx, deleteTagsCmd, repositoryID, manifestDigest); err != nil {
		return false, nil, err
	}

	// Delete the manifest itself from repository_objects.
	const deleteManifestCmd = `DELETE FROM repository_objects WHERE repository_id = $1 AND digest = $2`
	if _, err := tx.Exec(ctx, deleteManifestCmd, repositoryID, manifestDigest); err != nil {
		return false, nil, err
	}

	// Delete any child manifests (e.g. platform images inside a manifest index)
	// that now have no other parent remaining in this repository. We only go one
	// level deep since the OCI spec does not nest manifest indexes.
	const deleteOrphanChildManifestsCmd = `DELETE FROM repository_objects
	WHERE repository_id = $1
	  AND digest IN (
		SELECT g.child_digest
		FROM graph g
		JOIN objects o ON o.digest = g.child_digest
		WHERE g.parent_digest = $2
		  AND o.type IN ('manifest', 'manifest_index')
		  AND NOT EXISTS (
			SELECT 1 FROM graph g2
			JOIN repository_objects ro2 ON ro2.digest = g2.parent_digest
			WHERE g2.child_digest = g.child_digest
			  AND ro2.repository_id = $1
		  )
	  )
	RETURNING digest`

	childRows, err := tx.Query(ctx, deleteOrphanChildManifestsCmd, repositoryID, manifestDigest)
	if err != nil {
		return false, nil, err
	}
	deletedManifestDigests := []string{manifestDigest}
	for childRows.Next() {
		var d string
		if err := childRows.Scan(&d); err != nil {
			childRows.Close()
			return false, nil, err
		}
		deletedManifestDigests = append(deletedManifestDigests, d)
	}
	childRows.Close()
	if err := childRows.Err(); err != nil {
		return false, nil, err
	}

	// Remove blobs from repository_objects for this repo when they were
	// children of the deleted manifests and are no longer referenced by any
	// surviving manifest in this repository. Leave them for global GC otherwise.
	const deleteBlobsCmd = `DELETE FROM repository_objects
	WHERE repository_id = $1
	  AND digest IN (
		SELECT g.child_digest
		FROM graph g
		JOIN objects o ON o.digest = g.child_digest
		WHERE g.parent_digest = ANY($2)
		  AND o.type = 'blob'
	  )
	  AND NOT EXISTS (
		SELECT 1 FROM graph g2
		JOIN repository_objects ro2 ON ro2.digest = g2.parent_digest
		WHERE g2.child_digest = repository_objects.digest
		  AND ro2.repository_id = $1
	  )
	RETURNING digest`

	blobRows, err := tx.Query(ctx, deleteBlobsCmd, repositoryID, deletedManifestDigests)
	if err != nil {
		return false, nil, err
	}
	var removedBlobDigests []string
	for blobRows.Next() {
		var d string
		if err := blobRows.Scan(&d); err != nil {
			blobRows.Close()
			return false, nil, err
		}
		removedBlobDigests = append(removedBlobDigests, d)
	}
	blobRows.Close()
	if err := blobRows.Err(); err != nil {
		return false, nil, err
	}

	if len(removedBlobDigests) == 0 {
		// Committed with manifest removed but no blobs orphaned at tenant level.
		if err := tx.Commit(ctx); err != nil {
			return false, nil, err
		}
		return true, nil, nil
	}

	// Among blobs removed from this repo, find those now orphaned at the tenant
	// level (not referenced by any other repository belonging to this tenant).
	const tenantOrphanCmd = `SELECT o.digest, o.size_bytes
		FROM objects o
		WHERE o.digest = ANY($1)
		  AND NOT EXISTS (
			SELECT 1 FROM repository_objects ro2
			JOIN repositories r2  ON ro2.repository_id = r2.id
			JOIN registries   reg ON r2.registry_id    = reg.id
			WHERE ro2.digest = o.digest AND reg.tenant_id = $2
		  )`
	orphanRows, err := tx.Query(ctx, tenantOrphanCmd, removedBlobDigests, tenantID)
	if err != nil {
		return false, nil, err
	}
	var orphaned []DeletedBlobInfo
	for orphanRows.Next() {
		var info DeletedBlobInfo
		if err := orphanRows.Scan(&info.Digest, &info.SizeBytes); err != nil {
			orphanRows.Close()
			return false, nil, err
		}
		orphaned = append(orphaned, info)
	}
	orphanRows.Close()
	if err := orphanRows.Err(); err != nil {
		return false, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, nil, err
	}
	return true, orphaned, nil
}

func (d *DB) ListUnreferencedObjectDigests(ctx context.Context, limit int, minAge time.Duration) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	cutoff := time.Now().Add(-minAge)
	const cmd = `WITH RECURSIVE reachable AS (
		SELECT DISTINCT digest FROM tags
		UNION
		SELECT g.child_digest
		FROM graph g
		JOIN reachable r ON r.digest = g.parent_digest
	)
	SELECT o.digest
	FROM objects o
	LEFT JOIN reachable r ON r.digest = o.digest
	WHERE r.digest IS NULL
	  AND o.created_at < $2
	  AND (o.existence_checked_at IS NULL OR o.existence_checked_at < $2)
	LIMIT $1`
	rows, err := d.conn.Query(ctx, cmd, limit, cutoff)
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
	return digests, rows.Err()
}

func (d *DB) DeleteObject(ctx context.Context, digest string) error {
	const cmd = `DELETE FROM objects WHERE digest = $1`
	_, err := d.conn.Exec(ctx, cmd, strings.TrimSpace(digest))
	return err
}

// GCDiagnostics holds counts useful for understanding why GC collected 0 objects.
type GCDiagnostics struct {
	TotalObjects      int
	ReachableObjects  int
	UnreachableObjects int
	EligibleObjects   int // unreachable AND older than minAge
}

// DiagnoseGC returns object counts at each stage of the GC filter pipeline.
func (d *DB) DiagnoseGC(ctx context.Context, minAge time.Duration) (GCDiagnostics, error) {
	cutoff := time.Now().Add(-minAge)

	var diag GCDiagnostics

	if err := d.conn.QueryRow(ctx, `SELECT COUNT(*) FROM objects`).Scan(&diag.TotalObjects); err != nil {
		return diag, fmt.Errorf("count objects: %w", err)
	}

	const reachableCmd = `WITH RECURSIVE reachable AS (
		SELECT DISTINCT digest FROM tags
		UNION
		SELECT g.child_digest
		FROM graph g
		JOIN reachable r ON r.digest = g.parent_digest
	)
	SELECT COUNT(*) FROM reachable`
	if err := d.conn.QueryRow(ctx, reachableCmd).Scan(&diag.ReachableObjects); err != nil {
		return diag, fmt.Errorf("count reachable: %w", err)
	}

	diag.UnreachableObjects = diag.TotalObjects - diag.ReachableObjects

	const eligibleCmd = `WITH RECURSIVE reachable AS (
		SELECT DISTINCT digest FROM tags
		UNION
		SELECT g.child_digest
		FROM graph g
		JOIN reachable r ON r.digest = g.parent_digest
	)
	SELECT COUNT(*)
	FROM objects o
	LEFT JOIN reachable r ON r.digest = o.digest
	WHERE r.digest IS NULL
	  AND o.created_at < $1
	  AND (o.existence_checked_at IS NULL OR o.existence_checked_at < $1)`
	if err := d.conn.QueryRow(ctx, eligibleCmd, cutoff).Scan(&diag.EligibleObjects); err != nil {
		return diag, fmt.Errorf("count eligible: %w", err)
	}

	return diag, nil
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
	const cmd = `SELECT id FROM repositories WHERE registry_id = $1 AND name = $2 LIMIT 1`
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
