package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"bin2.io/internal/db"
	"github.com/google/uuid"
)

// capturedEvent is a single usage event recorded by memDB.
type capturedEvent struct {
	metric string
	value  int64
	digest string
}

// --------------------------------------------------------------------------
// In-memory registry storage backend
// --------------------------------------------------------------------------

type memStorage struct {
	mu      sync.Mutex
	blobs   map[string][]byte        // digestHex → content
	uploads map[string]*bytes.Buffer // uploadID → buffer
}

func newMemStorage() *memStorage {
	return &memStorage{
		blobs:   make(map[string][]byte),
		uploads: make(map[string]*bytes.Buffer),
	}
}

func (s *memStorage) BlobExists(_ context.Context, digestHex string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.blobs[digestHex]
	return ok, nil
}

func (s *memStorage) BlobSize(_ context.Context, digestHex string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.blobs[digestHex]
	if !ok {
		return 0, ErrBlobNotFound
	}
	return int64(len(b)), nil
}

func (s *memStorage) GetBlob(_ context.Context, digestHex string) (io.ReadCloser, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.blobs[digestHex]
	if !ok {
		return nil, 0, ErrBlobNotFound
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return io.NopCloser(bytes.NewReader(cp)), int64(len(cp)), nil
}

func (s *memStorage) DeleteBlob(_ context.Context, digestHex string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.blobs, digestHex)
	return nil
}

func (s *memStorage) CreateUpload(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploads[id] = &bytes.Buffer{}
	return nil
}

func (s *memStorage) AppendUpload(_ context.Context, id string, body io.Reader) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, ok := s.uploads[id]
	if !ok {
		return 0, ErrUploadNotFound
	}
	if _, err := io.Copy(buf, body); err != nil {
		return int64(buf.Len()), err
	}
	return int64(buf.Len()), nil
}

func (s *memStorage) UploadSize(_ context.Context, id string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, ok := s.uploads[id]
	if !ok {
		return 0, ErrUploadNotFound
	}
	return int64(buf.Len()), nil
}

func (s *memStorage) UploadSHA256(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, ok := s.uploads[id]
	if !ok {
		return "", ErrUploadNotFound
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

func (s *memStorage) StoreBlobFromUpload(_ context.Context, id string, digestHex string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, ok := s.uploads[id]
	if !ok {
		return 0, ErrUploadNotFound
	}
	cp := make([]byte, buf.Len())
	copy(cp, buf.Bytes())
	s.blobs[digestHex] = cp
	delete(s.uploads, id)
	return int64(len(cp)), nil
}

func (s *memStorage) DeleteUpload(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.uploads, id)
	return nil
}

// --------------------------------------------------------------------------
// In-memory registry database
// --------------------------------------------------------------------------

type memManifestEntry struct {
	body          []byte
	contentType   string
	subjectDigest string
}

type memDB struct {
	mu uuid.UUID // tenantID shared by all registries in this instance

	rmu        sync.Mutex
	registries map[string]db.Registry // name → Registry

	omu           sync.Mutex
	blobs         map[string]int64          // digest → size
	repoObjs      map[string]bool           // repoObjKey → present
	manifests     map[string]memManifestEntry // manifestKey → entry
	manifestBlobs map[string][]string       // manifestKey → []blobDigest
	tags          map[string]string         // tagKey → digest
	// graph: "registryID\x00repo\x00parentDigest\x00childDigest" → true
	graph  map[string]bool
	events []capturedEvent // captured InsertUsageEvents calls
}

func newMemDB(registryName string, tenantID uuid.UUID) *memDB {
	registryID := uuid.New()
	m := &memDB{
		mu:            tenantID,
		registries:    make(map[string]db.Registry),
		blobs:         make(map[string]int64),
		repoObjs:      make(map[string]bool),
		manifests:     make(map[string]memManifestEntry),
		manifestBlobs: make(map[string][]string),
		tags:          make(map[string]string),
		graph:         make(map[string]bool),
	}
	m.registries[registryName] = db.Registry{
		ID:       registryID,
		TenantID: tenantID,
		Name:     registryName,
	}
	return m
}

func (m *memDB) rk(registryID uuid.UUID, repo, digest string) string {
	return fmt.Sprintf("%s\x00%s\x00%s", registryID, repo, digest)
}

func (m *memDB) tk(registryID uuid.UUID, repo, tag string) string {
	return fmt.Sprintf("%s\x00%s\x00%s", registryID, repo, tag)
}

func (m *memDB) GetRegistryByName(_ context.Context, name string) (db.Registry, error) {
	m.rmu.Lock()
	defer m.rmu.Unlock()
	r, ok := m.registries[name]
	if !ok {
		return db.Registry{}, db.ErrNotFound
	}
	return r, nil
}

func (m *memDB) GetRegistryTenantID(_ context.Context, registryID uuid.UUID) (uuid.UUID, error) {
	m.rmu.Lock()
	defer m.rmu.Unlock()
	for _, r := range m.registries {
		if r.ID == registryID {
			return r.TenantID, nil
		}
	}
	return uuid.Nil, db.ErrNotFound
}

func (m *memDB) TenantHasBlob(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return false, nil
}

func (m *memDB) GetRepositoryObjectSize(_ context.Context, registryID uuid.UUID, repository, digest string) (int64, error) {
	m.omu.Lock()
	defer m.omu.Unlock()
	if !m.repoObjs[m.rk(registryID, repository, digest)] {
		return 0, db.ErrNotFound
	}
	size, ok := m.blobs[digest]
	if !ok {
		return 0, db.ErrNotFound
	}
	return size, nil
}

func (m *memDB) UpsertObjectBlob(_ context.Context, digest string, sizeBytes int64) error {
	m.omu.Lock()
	defer m.omu.Unlock()
	m.blobs[digest] = sizeBytes
	return nil
}

func (m *memDB) GetObjectSize(_ context.Context, digest string) (int64, error) {
	m.omu.Lock()
	defer m.omu.Unlock()
	size, ok := m.blobs[digest]
	if !ok {
		return 0, db.ErrNotFound
	}
	return size, nil
}

func (m *memDB) NoteObjectExistenceCheck(_ context.Context, _ string) error {
	return nil
}

func (m *memDB) InsertUsageEvents(_ context.Context, events []db.UsageEvent) error {
	m.omu.Lock()
	defer m.omu.Unlock()
	for _, e := range events {
		m.events = append(m.events, capturedEvent{
			metric: e.Metric,
			value:  e.Value,
			digest: e.Digest,
		})
	}
	return nil
}

func (m *memDB) UpsertManifest(_ context.Context, args db.UpsertManifestArgs) error {
	m.omu.Lock()
	defer m.omu.Unlock()

	// Record manifest body
	mkey := m.rk(args.RegistryID, args.Repository, args.ManifestDigest)
	m.manifests[mkey] = memManifestEntry{
		body:          args.ManifestBody,
		contentType:   args.ContentType,
		subjectDigest: args.SubjectDigest,
	}

	// Add manifest and blobs to repo objects
	m.repoObjs[m.rk(args.RegistryID, args.Repository, args.ManifestDigest)] = true
	for _, bd := range args.BlobDigests {
		m.repoObjs[m.rk(args.RegistryID, args.Repository, bd)] = true
	}

	// Record which blobs belong to this manifest (for orphan detection on delete)
	if len(args.BlobDigests) > 0 {
		cp := make([]string, len(args.BlobDigests))
		copy(cp, args.BlobDigests)
		m.manifestBlobs[mkey] = cp
	}

	// Record parent→child edges for manifest indexes
	for _, child := range args.ChildManifestDigests {
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", args.RegistryID, args.Repository, args.ManifestDigest, child)
		m.graph[key] = true
	}

	// Update tag
	if args.Tag != "" {
		m.tags[m.tk(args.RegistryID, args.Repository, args.Tag)] = args.ManifestDigest
	}
	return nil
}

func (m *memDB) HasManifestDigestInRepository(_ context.Context, registryID uuid.UUID, repository, manifestDigest string) (bool, error) {
	m.omu.Lock()
	defer m.omu.Unlock()
	return m.repoObjs[m.rk(registryID, repository, manifestDigest)], nil
}

func (m *memDB) GetManifestByReference(_ context.Context, registryID uuid.UUID, repository, reference string) ([]byte, string, string, error) {
	m.omu.Lock()
	defer m.omu.Unlock()

	digest := reference
	if !strings.HasPrefix(reference, "sha256:") {
		digest = m.tags[m.tk(registryID, repository, reference)]
		if digest == "" {
			return nil, "", "", db.ErrNotFound
		}
	}

	entry, ok := m.manifests[m.rk(registryID, repository, digest)]
	if !ok {
		return nil, "", "", db.ErrNotFound
	}
	return entry.body, entry.contentType, digest, nil
}

func (m *memDB) DeleteManifestByDigestInRepository(_ context.Context, registryID uuid.UUID, _ uuid.UUID, repository, manifestDigest string) (bool, []db.DeletedBlobInfo, error) {
	m.omu.Lock()
	defer m.omu.Unlock()

	mkey := m.rk(registryID, repository, manifestDigest)
	if _, ok := m.manifests[mkey]; !ok {
		return false, nil, nil
	}

	// Check if any still-present manifest in the repo references this as a child.
	repoPrefix := fmt.Sprintf("%s\x00%s\x00", registryID, repository)
	childSuffix := "\x00" + manifestDigest
	for edge := range m.graph {
		if !strings.HasPrefix(edge, repoPrefix) {
			continue
		}
		if !strings.HasSuffix(edge, childSuffix) {
			continue
		}
		// Extract parent digest: "registryID\x00repo\x00parent\x00child"
		rest := strings.TrimPrefix(edge, repoPrefix)
		parentDigest := strings.TrimSuffix(rest, childSuffix)
		if m.repoObjs[m.rk(registryID, repository, parentDigest)] {
			return false, nil, db.ErrManifestHasParent
		}
	}

	// Collect blobs referenced by this manifest before deleting.
	blobsForManifest := m.manifestBlobs[mkey]

	delete(m.manifests, mkey)
	delete(m.repoObjs, mkey)
	delete(m.manifestBlobs, mkey)

	// Remove any tags pointing to this digest
	for tk, td := range m.tags {
		if strings.HasPrefix(tk, repoPrefix) && td == manifestDigest {
			delete(m.tags, tk)
		}
	}

	// Compute tenant-level orphans: blobs no longer referenced by any remaining
	// manifest in this registry instance.
	var orphaned []db.DeletedBlobInfo
	for _, blobDigest := range blobsForManifest {
		referenced := false
		for _, blobs := range m.manifestBlobs {
			for _, bd := range blobs {
				if bd == blobDigest {
					referenced = true
					break
				}
			}
			if referenced {
				break
			}
		}
		if !referenced {
			orphaned = append(orphaned, db.DeletedBlobInfo{
				Digest:    blobDigest,
				SizeBytes: m.blobs[blobDigest],
			})
		}
	}
	return true, orphaned, nil
}

func (m *memDB) DeleteManifestReference(_ context.Context, registryID uuid.UUID, repository, reference string) (bool, error) {
	m.omu.Lock()
	defer m.omu.Unlock()

	tkey := m.tk(registryID, repository, reference)
	if _, ok := m.tags[tkey]; !ok {
		return false, nil
	}
	delete(m.tags, tkey)
	return true, nil
}

func (m *memDB) ListRepositoryManifestRecords(_ context.Context, registryID uuid.UUID, repository string) ([]db.RepositoryManifestRecord, error) {
	m.omu.Lock()
	defer m.omu.Unlock()

	prefix := fmt.Sprintf("%s\x00%s\x00", registryID, repository)
	records := make([]db.RepositoryManifestRecord, 0)
	for k, entry := range m.manifests {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		digest := strings.TrimPrefix(k, prefix)
		records = append(records, db.RepositoryManifestRecord{
			Digest:      digest,
			ContentType: entry.contentType,
			Size:        int64(len(entry.body)),
			Body:        entry.body,
		})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Digest < records[j].Digest
	})
	return records, nil
}

func (m *memDB) ListRepositoryTags(_ context.Context, registryID uuid.UUID, repository string, limit int, last string) ([]string, error) {
	m.omu.Lock()
	defer m.omu.Unlock()

	if limit <= 0 {
		limit = 100
	}

	prefix := fmt.Sprintf("%s\x00%s\x00", registryID, repository)
	all := make([]string, 0)
	for k := range m.tags {
		if strings.HasPrefix(k, prefix) {
			all = append(all, strings.TrimPrefix(k, prefix))
		}
	}
	// Case-insensitive sort, case-sensitive tiebreak (mirrors the SQL query)
	sort.Slice(all, func(i, j int) bool {
		li, lj := strings.ToLower(all[i]), strings.ToLower(all[j])
		if li != lj {
			return li < lj
		}
		return all[i] < all[j]
	})

	// Apply cursor
	lastLower := strings.ToLower(last)
	out := make([]string, 0, limit)
	for _, tag := range all {
		if last != "" {
			tl := strings.ToLower(tag)
			if tl < lastLower || (tl == lastLower && tag <= last) {
				continue
			}
		}
		out = append(out, tag)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
