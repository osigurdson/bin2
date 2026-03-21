package server

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// --------------------------------------------------------------------------
// Test server setup
// --------------------------------------------------------------------------

type testRegistryEnv struct {
	server     *Server
	db         *memDB
	storage    *memStorage
	namespace  string
	registryID uuid.UUID
}

func newTestRegistryEnv(t *testing.T, namespace string) *testRegistryEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	tenantID := uuid.New()
	mdb := newMemDB(namespace, tenantID)
	storage := newMemStorage()

	reg, err := mdb.GetRegistryByName(nil, namespace)
	if err != nil {
		t.Fatalf("GetRegistryByName: %v", err)
	}

	s := &Server{
		router:                gin.New(),
		registryJWTPrivateKey: privKey,
		registryJWTPublicKey:  pubKey,
		registryService:       "registry.test",
		registryStorage:       storage,
		registryDB:            mdb,
		probeCache:            &probeCache{recent: make(map[string]time.Time)},
	}
	s.addRegistryRoutes()

	return &testRegistryEnv{
		server:     s,
		db:         mdb,
		storage:    storage,
		namespace:  namespace,
		registryID: reg.ID,
	}
}

// token issues a Bearer token scoped to the given repository actions.
func (e *testRegistryEnv) token(t *testing.T, repo string, actions ...string) string {
	t.Helper()
	tok, _, _, err := e.server.issueRegistryToken(e.namespace, "registry.test", []registryTokenAccess{{
		Type:    "repository",
		Name:    repo,
		Actions: actions,
	}})
	if err != nil {
		t.Fatalf("issueRegistryToken: %v", err)
	}
	return tok
}

// do sends a request to the test server and returns the response.
func (e *testRegistryEnv) do(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.server.router.ServeHTTP(w, req)
	return w
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// pushBlob performs a chunked blob upload (POST → PATCH → PUT) and returns the
// digest string ("sha256:…"). It asserts each step succeeds.
func (e *testRegistryEnv) pushBlob(t *testing.T, repo string, content []byte) string {
	t.Helper()
	tok := e.token(t, repo, "push")

	// POST — start upload
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v2/%s/blobs/uploads/", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST blobs/uploads: status %d, body: %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("POST blobs/uploads: no Location header")
	}

	// PATCH — send content
	req = httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.ContentLength = int64(len(content))
	w = e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("PATCH upload: status %d, body: %s", w.Code, w.Body.String())
	}
	location = w.Header().Get("Location")

	// PUT — finalize
	digestStr := "sha256:" + sha256hex(content)
	putURL := location + "?digest=" + digestStr
	req = httptest.NewRequest(http.MethodPut, putURL, http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PUT finalize: status %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Docker-Content-Digest"); got != digestStr {
		t.Fatalf("PUT finalize: Docker-Content-Digest = %q, want %q", got, digestStr)
	}
	return digestStr
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestBlobUploadChunked(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/app"
	content := []byte("hello integration test blob")

	digest := e.pushBlob(t, repo, content)

	// After pushing a manifest that references this blob, HEAD should work.
	// For now just verify the blob is in storage.
	digestHex := strings.TrimPrefix(digest, "sha256:")
	exists, err := e.storage.BlobExists(nil, digestHex)
	if err != nil || !exists {
		t.Fatalf("blob not in storage after push: err=%v exists=%v", err, exists)
	}
}

func TestBlobUploadMonolithic(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/app"
	content := []byte("monolithic blob content")
	digestStr := "sha256:" + sha256hex(content)

	tok := e.token(t, repo, "push")
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/v2/%s/blobs/uploads/?digest=%s", repo, digestStr),
		bytes.NewReader(content),
	)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.ContentLength = int64(len(content))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("monolithic upload: status %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Docker-Content-Digest"); got != digestStr {
		t.Fatalf("Docker-Content-Digest = %q, want %q", got, digestStr)
	}
}

func TestManifestPushPull(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/myapp"

	// Push config and layer blobs
	configContent := []byte(`{}`)
	layerContent := []byte("layer data")
	configDigest := e.pushBlob(t, repo, configContent)
	layerDigest := e.pushBlob(t, repo, layerContent)

	// Construct a minimal OCI image manifest
	manifest := imageManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      int64(len(configContent)),
		},
		Layers: []descriptor{{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    layerDigest,
			Size:      int64(len(layerContent)),
		}},
	}
	manifestBytes := mustJSON(manifest)
	manifestDigest := "sha256:" + sha256hex(manifestBytes)

	tok := e.token(t, repo, "push", "pull")
	contentType := "application/vnd.oci.image.manifest.v1+json"

	// PUT /v2/alpha/myapp/manifests/latest
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/latest", repo), bytes.NewReader(manifestBytes))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(manifestBytes))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PUT manifest: status %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Docker-Content-Digest"); got != manifestDigest {
		t.Fatalf("Docker-Content-Digest = %q, want %q", got, manifestDigest)
	}

	// GET /v2/alpha/myapp/manifests/latest — by tag
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/latest", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET manifest by tag: status %d", w.Code)
	}
	if got := w.Header().Get("Docker-Content-Digest"); got != manifestDigest {
		t.Fatalf("GET manifest by tag: Docker-Content-Digest = %q, want %q", got, manifestDigest)
	}
	if !bytes.Equal(w.Body.Bytes(), manifestBytes) {
		t.Fatalf("GET manifest by tag: body mismatch")
	}

	// GET /v2/alpha/myapp/manifests/<digest> — by digest
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/%s", repo, manifestDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET manifest by digest: status %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), manifestBytes) {
		t.Fatalf("GET manifest by digest: body mismatch")
	}

	// HEAD /v2/alpha/myapp/manifests/latest
	req = httptest.NewRequest(http.MethodHead, fmt.Sprintf("/v2/%s/manifests/latest", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("HEAD manifest: status %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("HEAD manifest: non-empty body")
	}
	if got := w.Header().Get("Content-Length"); got == "" {
		t.Fatal("HEAD manifest: missing Content-Length")
	}

	// HEAD /v2/alpha/myapp/blobs/<configDigest> — only works after manifest push
	req = httptest.NewRequest(http.MethodHead, fmt.Sprintf("/v2/%s/blobs/%s", repo, configDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("HEAD blob after manifest push: status %d", w.Code)
	}
	if got := w.Header().Get("Docker-Content-Digest"); got != configDigest {
		t.Fatalf("HEAD blob: Docker-Content-Digest = %q, want %q", got, configDigest)
	}

	// GET /v2/alpha/myapp/blobs/<layerDigest>
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/blobs/%s", repo, layerDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET blob: status %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), layerContent) {
		t.Fatalf("GET blob: content mismatch")
	}
}

func TestTagList(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/tagrepo"

	// Push a single blob to reuse across all manifests
	configContent := []byte(`{}`)
	layerContent := []byte("shared layer")
	configDigest := e.pushBlob(t, repo, configContent)
	layerDigest := e.pushBlob(t, repo, layerContent)

	tok := e.token(t, repo, "push", "pull")
	contentType := "application/vnd.oci.image.manifest.v1+json"

	pushTag := func(tag string) {
		t.Helper()
		// Use a unique annotation per tag so each manifest has a distinct digest
		manifest := imageManifest{
			SchemaVersion: 2,
			MediaType:     contentType,
			Config: descriptor{
				MediaType: "application/vnd.oci.image.config.v1+json",
				Digest:    configDigest,
				Size:      int64(len(configContent)),
			},
			Layers: []descriptor{{
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Digest:    layerDigest,
				Size:      int64(len(layerContent)),
			}},
			Annotations: map[string]string{"tag": tag},
		}
		body := mustJSON(manifest)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/%s", repo, tag), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", contentType)
		req.ContentLength = int64(len(body))
		w := e.do(req)
		if w.Code != http.StatusCreated {
			t.Fatalf("push tag %q: status %d, body: %s", tag, w.Code, w.Body.String())
		}
	}

	tags := []string{"v1.0", "v1.1", "v2.0", "latest", "beta"}
	for _, tag := range tags {
		pushTag(tag)
	}

	// GET /v2/alpha/tagrepo/tags/list
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/tags/list", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("tags/list: status %d, body: %s", w.Code, w.Body.String())
	}

	var resp tagListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("tags/list: unmarshal: %v", err)
	}
	if len(resp.Tags) != len(tags) {
		t.Fatalf("tags/list: got %d tags, want %d: %v", len(resp.Tags), len(tags), resp.Tags)
	}

	// Verify pagination with n=2
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/tags/list?n=2", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("tags/list?n=2: status %d", w.Code)
	}
	var page1 tagListResponse
	json.Unmarshal(w.Body.Bytes(), &page1)
	if len(page1.Tags) != 2 {
		t.Fatalf("tags/list?n=2: got %d tags, want 2: %v", len(page1.Tags), page1.Tags)
	}

	// Second page using last= cursor
	lastTag := page1.Tags[len(page1.Tags)-1]
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/tags/list?n=2&last=%s", repo, lastTag), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("tags/list page2: status %d", w.Code)
	}
	var page2 tagListResponse
	json.Unmarshal(w.Body.Bytes(), &page2)
	if len(page2.Tags) != 2 {
		t.Fatalf("tags/list page2: got %d tags, want 2: %v", len(page2.Tags), page2.Tags)
	}
	// Ensure no overlap
	for _, tag := range page1.Tags {
		for _, tag2 := range page2.Tags {
			if tag == tag2 {
				t.Fatalf("tags/list pagination: duplicate tag %q across pages", tag)
			}
		}
	}
}

func TestReferrers(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/refs"

	// Push blobs
	configContent := []byte(`{}`)
	layerContent := []byte("referrers test layer")
	configDigest := e.pushBlob(t, repo, configContent)
	layerDigest := e.pushBlob(t, repo, layerContent)

	tok := e.token(t, repo, "push", "pull")
	contentType := "application/vnd.oci.image.manifest.v1+json"

	// Push subject manifest
	subject := imageManifest{
		SchemaVersion: 2,
		MediaType:     contentType,
		Config: descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      int64(len(configContent)),
		},
		Layers: []descriptor{{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    layerDigest,
			Size:      int64(len(layerContent)),
		}},
	}
	subjectBytes := mustJSON(subject)
	subjectDigest := "sha256:" + sha256hex(subjectBytes)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/subject", repo), bytes.NewReader(subjectBytes))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(subjectBytes))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push subject manifest: status %d, body: %s", w.Code, w.Body.String())
	}

	// Push an empty config blob for the referrer
	emptyConfig := []byte("{}")
	emptyConfigDigest := e.pushBlob(t, repo, emptyConfig)

	// Push referrer manifest pointing at subject
	referrer := imageManifest{
		SchemaVersion: 2,
		MediaType:     contentType,
		ArtifactType:  "application/vnd.example.sbom",
		Config: descriptor{
			MediaType: "application/vnd.oci.empty.v1+json",
			Digest:    emptyConfigDigest,
			Size:      int64(len(emptyConfig)),
		},
		Layers: []descriptor{},
		Subject: &descriptor{
			MediaType: contentType,
			Digest:    subjectDigest,
			Size:      int64(len(subjectBytes)),
		},
	}
	referrerBytes := mustJSON(referrer)

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/referrer", repo), bytes.NewReader(referrerBytes))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(referrerBytes))
	w = e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push referrer manifest: status %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("OCI-Subject"); got != subjectDigest {
		t.Fatalf("push referrer: OCI-Subject = %q, want %q", got, subjectDigest)
	}

	// GET /v2/alpha/refs/referrers/<subjectDigest>
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/referrers/%s", repo, subjectDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET referrers: status %d, body: %s", w.Code, w.Body.String())
	}

	var idx imageIndex
	if err := json.Unmarshal(w.Body.Bytes(), &idx); err != nil {
		t.Fatalf("GET referrers: unmarshal: %v", err)
	}
	if len(idx.Manifests) != 1 {
		t.Fatalf("GET referrers: got %d manifests, want 1", len(idx.Manifests))
	}
	if idx.Manifests[0].ArtifactType != "application/vnd.example.sbom" {
		t.Fatalf("GET referrers: ArtifactType = %q, want application/vnd.example.sbom", idx.Manifests[0].ArtifactType)
	}

	// Filter by artifactType — matching
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/referrers/%s?artifactType=application%%2Fvnd.example.sbom", repo, subjectDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET referrers filtered: status %d", w.Code)
	}
	json.Unmarshal(w.Body.Bytes(), &idx)
	if len(idx.Manifests) != 1 {
		t.Fatalf("GET referrers filtered: got %d manifests, want 1", len(idx.Manifests))
	}
	if got := w.Header().Get("OCI-Filters-Applied"); got != "artifactType" {
		t.Fatalf("GET referrers filtered: OCI-Filters-Applied = %q, want artifactType", got)
	}

	// Filter by artifactType — non-matching
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/referrers/%s?artifactType=other%%2Ftype", repo, subjectDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET referrers non-match: status %d", w.Code)
	}
	json.Unmarshal(w.Body.Bytes(), &idx)
	if len(idx.Manifests) != 0 {
		t.Fatalf("GET referrers non-match: got %d manifests, want 0", len(idx.Manifests))
	}
}

func TestManifestDelete(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/delrepo"

	configContent := []byte(`{}`)
	layerContent := []byte("delete test layer")
	configDigest := e.pushBlob(t, repo, configContent)
	layerDigest := e.pushBlob(t, repo, layerContent)

	tok := e.token(t, repo, "push", "pull")
	contentType := "application/vnd.oci.image.manifest.v1+json"

	manifest := imageManifest{
		SchemaVersion: 2,
		MediaType:     contentType,
		Config: descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      int64(len(configContent)),
		},
		Layers: []descriptor{{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    layerDigest,
			Size:      int64(len(layerContent)),
		}},
	}
	manifestBytes := mustJSON(manifest)
	manifestDigest := "sha256:" + sha256hex(manifestBytes)

	// Push
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/v1.0", repo), bytes.NewReader(manifestBytes))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(manifestBytes))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push manifest: status %d", w.Code)
	}

	// DELETE by tag — should delete the tag reference only
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/v1.0", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("DELETE by tag: status %d, body: %s", w.Code, w.Body.String())
	}

	// GET by tag should now 404
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/v1.0", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET after tag delete: status %d, want 404", w.Code)
	}

	// GET by digest should still work (tag delete doesn't remove manifest)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/%s", repo, manifestDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET by digest after tag delete: status %d", w.Code)
	}

	// DELETE by digest — removes the manifest entirely
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/%s", repo, manifestDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("DELETE by digest: status %d, body: %s", w.Code, w.Body.String())
	}

	// GET by digest should now 404
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/%s", repo, manifestDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET after digest delete: status %d, want 404", w.Code)
	}
}

// TestBlobUploadStatusCheck verifies GET on an in-progress upload session returns
// 204 with a Range header showing the current byte offset.
func TestBlobUploadStatusCheck(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/statusrepo"
	tok := e.token(t, repo, "push")

	// Start upload
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v2/%s/blobs/uploads/", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST blobs/uploads: status %d", w.Code)
	}
	location := w.Header().Get("Location")

	// GET on the session before any data is sent → 204, Range: 0-0
	req = httptest.NewRequest(http.MethodGet, location, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("GET upload status (empty): status %d, want 204", w.Code)
	}
	if got := w.Header().Get("Range"); got != "0-0" {
		t.Fatalf("GET upload status (empty): Range = %q, want 0-0", got)
	}

	// PATCH some data
	content := []byte("partial data")
	req = httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.ContentLength = int64(len(content))
	w = e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("PATCH upload: status %d", w.Code)
	}
	location = w.Header().Get("Location")

	// GET again → 204, Range reflects bytes received
	req = httptest.NewRequest(http.MethodGet, location, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("GET upload status (after patch): status %d, want 204", w.Code)
	}
	wantRange := fmt.Sprintf("0-%d", len(content)-1)
	if got := w.Header().Get("Range"); got != wantRange {
		t.Fatalf("GET upload status (after patch): Range = %q, want %q", got, wantRange)
	}
}

// TestBlobUploadOutOfOrderChunk verifies that sending a Content-Range starting
// at the wrong offset returns 416 Range Not Satisfiable.
func TestBlobUploadOutOfOrderChunk(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/rangerepo"
	tok := e.token(t, repo, "push")

	// Start upload
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v2/%s/blobs/uploads/", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST blobs/uploads: status %d", w.Code)
	}
	location := w.Header().Get("Location")

	// PATCH with Content-Range starting at byte 5 but upload is at 0 → 416
	content := []byte("some content")
	req = httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Range", fmt.Sprintf("5-%d", 5+len(content)-1))
	req.ContentLength = int64(len(content))
	w = e.do(req)
	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("out-of-order PATCH: status %d, want 416", w.Code)
	}

	// The Range header in the response should reflect the actual current offset (0)
	if got := w.Header().Get("Range"); got != "0-0" {
		t.Fatalf("out-of-order PATCH: Range = %q, want 0-0", got)
	}
}

// TestNamespaceMismatch verifies that requests to a different namespace are
// rejected. There are two distinct enforcement layers:
//
//  1. The Bearer middleware rejects with 401 when the token has no scope for the
//     target repository (the common case: token scoped to alpha can't reach beta).
//
//  2. ensureRepoAuthorized rejects with 403 when the middleware passes because the
//     token explicitly carries scope for the target repo, but the token's subject
//     namespace doesn't match the repo's namespace prefix.
func TestNamespaceMismatch(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")

	// Case 1: token scoped to alpha/app, request targets beta/app.
	// The middleware sees no beta/app scope in the token → 401 DENIED.
	tok := e.token(t, "alpha/app", "push", "pull")

	req := httptest.NewRequest(http.MethodPost, "/v2/beta/app/blobs/uploads/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("cross-namespace POST (no scope): status %d, want 401", w.Code)
	}
	if !strings.Contains(w.Body.String(), "DENIED") {
		t.Fatalf("cross-namespace POST: body %q does not contain DENIED", w.Body.String())
	}

	// Case 2: token subject is "alpha" but has been issued with beta/app scope
	// (simulates a crafted or misconfigured token). The middleware passes, but
	// ensureRepoAuthorized catches the namespace mismatch → 403 DENIED.
	crossTok, _, _, err := e.server.issueRegistryToken("alpha", "registry.test", []registryTokenAccess{{
		Type:    "repository",
		Name:    "beta/app",
		Actions: []string{"push", "pull"},
	}})
	if err != nil {
		t.Fatalf("issueRegistryToken: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v2/beta/app/blobs/uploads/", nil)
	req.Header.Set("Authorization", "Bearer "+crossTok)
	w = e.do(req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-namespace POST (subject mismatch): status %d, want 403", w.Code)
	}
	if !strings.Contains(w.Body.String(), "DENIED") {
		t.Fatalf("cross-namespace POST (subject mismatch): body %q does not contain DENIED", w.Body.String())
	}
}

// TestDeleteChildOfIndexRejected verifies that deleting a manifest that is still
// referenced as a child of an image index returns 409 with MANIFEST_REFERENCED.
func TestDeleteChildOfIndexRejected(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/indexrepo"

	// Push blobs and child manifest
	configContent := []byte(`{}`)
	layerContent := []byte("index child layer")
	configDigest := e.pushBlob(t, repo, configContent)
	layerDigest := e.pushBlob(t, repo, layerContent)

	tok := e.token(t, repo, "push", "pull")
	manifestCT := "application/vnd.oci.image.manifest.v1+json"
	indexCT := "application/vnd.oci.image.index.v1+json"

	child := imageManifest{
		SchemaVersion: 2,
		MediaType:     manifestCT,
		Config: descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      int64(len(configContent)),
		},
		Layers: []descriptor{{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    layerDigest,
			Size:      int64(len(layerContent)),
		}},
	}
	childBytes := mustJSON(child)
	childDigest := "sha256:" + sha256hex(childBytes)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/child", repo), bytes.NewReader(childBytes))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", manifestCT)
	req.ContentLength = int64(len(childBytes))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push child: status %d, body: %s", w.Code, w.Body.String())
	}

	// Push an index referencing the child
	index := imageManifest{
		SchemaVersion: 2,
		MediaType:     indexCT,
		Manifests: []descriptor{{
			MediaType: manifestCT,
			Digest:    childDigest,
			Size:      int64(len(childBytes)),
		}},
	}
	indexBytes := mustJSON(index)

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/index", repo), bytes.NewReader(indexBytes))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", indexCT)
	req.ContentLength = int64(len(indexBytes))
	w = e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push index: status %d, body: %s", w.Code, w.Body.String())
	}

	// Attempt to delete the child while the index still references it → 409
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/%s", repo, childDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusConflict {
		t.Fatalf("DELETE child while referenced: status %d, want 409", w.Code)
	}
	if !strings.Contains(w.Body.String(), "MANIFEST_REFERENCED") {
		t.Fatalf("DELETE child while referenced: body %q does not contain MANIFEST_REFERENCED", w.Body.String())
	}

	// Deleting the index first should succeed
	indexDigest := "sha256:" + sha256hex(indexBytes)
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/%s", repo, indexDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("DELETE index: status %d, body: %s", w.Code, w.Body.String())
	}

	// Now deleting the child should succeed
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/%s", repo, childDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("DELETE child after index deleted: status %d, body: %s", w.Code, w.Body.String())
	}
}

// TestReferrersEmptyForUnknownDigest verifies that the referrers endpoint returns
// 200 with an empty manifests array for a digest that has no referrers, as
// required by the OCI spec.
func TestReferrersEmptyForUnknownDigest(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/emptyrefs"
	tok := e.token(t, repo, "pull")

	// A digest that was never pushed — referrers must still return 200 + empty list
	ghost := "sha256:" + strings.Repeat("0", 64)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/referrers/%s", repo, ghost), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET referrers for unknown digest: status %d, want 200", w.Code)
	}

	var idx imageIndex
	if err := json.Unmarshal(w.Body.Bytes(), &idx); err != nil {
		t.Fatalf("GET referrers for unknown digest: unmarshal: %v", err)
	}
	if len(idx.Manifests) != 0 {
		t.Fatalf("GET referrers for unknown digest: got %d manifests, want 0", len(idx.Manifests))
	}
	if idx.SchemaVersion != 2 {
		t.Fatalf("GET referrers for unknown digest: schemaVersion = %d, want 2", idx.SchemaVersion)
	}
}

// TestBlobSizeTrackedInDB verifies that after a blob is uploaded, the registry
// DB records its size so that GC tooling can account for storage consumption.
func TestBlobSizeTrackedInDB(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/gcrepo"
	content := []byte("blob for size tracking")

	digest := e.pushBlob(t, repo, content)

	size, err := e.db.GetObjectSize(nil, digest)
	if err != nil {
		t.Fatalf("GetObjectSize: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("GetObjectSize: got %d, want %d", size, len(content))
	}
}

// TestUsageEventEmittedOnManifestPush verifies that pushing a manifest emits a
// push-op-count usage event, which drives billing and GC accounting.
func TestUsageEventEmittedOnManifestPush(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/eventrepo"

	configDigest := e.pushBlob(t, repo, []byte(`{}`))
	layerDigest := e.pushBlob(t, repo, []byte("event test layer"))

	// Clear any events emitted during blob uploads so we only inspect the manifest push.
	e.db.omu.Lock()
	e.db.events = nil
	e.db.omu.Unlock()

	tok := e.token(t, repo, "push")
	manifest := imageManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      2,
		},
		Layers: []descriptor{{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    layerDigest,
			Size:      int64(len("event test layer")),
		}},
	}
	body := mustJSON(manifest)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/v1.0", repo), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
	req.ContentLength = int64(len(body))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PUT manifest: status %d, body: %s", w.Code, w.Body.String())
	}

	e.db.omu.Lock()
	captured := append([]capturedEvent(nil), e.db.events...)
	e.db.omu.Unlock()

	var found bool
	for _, ev := range captured {
		if ev.metric == "push-op-count" && ev.value == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no push-op-count event found; got events: %+v", captured)
	}
}

// TestOrphanedBlobAccountingOnDelete verifies that when a manifest is deleted
// and its blobs are no longer referenced by any other manifest, the server emits
// negative storage-bytes usage events for each orphaned blob.  A blob shared by
// a second manifest must NOT be reported as orphaned while that manifest exists.
func TestOrphanedBlobAccountingOnDelete(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/orphanrepo"

	sharedLayer := []byte("shared layer content")
	uniqueLayer := []byte("unique layer content")
	configA := []byte(`{"a":1}`)
	configB := []byte(`{"b":2}`)

	sharedDigest := e.pushBlob(t, repo, sharedLayer)
	uniqueDigest := e.pushBlob(t, repo, uniqueLayer)
	configADigest := e.pushBlob(t, repo, configA)
	configBDigest := e.pushBlob(t, repo, configB)

	tok := e.token(t, repo, "push")
	ct := "application/vnd.oci.image.manifest.v1+json"

	pushManifest := func(tag string, configDigest string, layers []descriptor) string {
		t.Helper()
		m := imageManifest{
			SchemaVersion: 2,
			MediaType:     ct,
			Config: descriptor{
				MediaType: "application/vnd.oci.image.config.v1+json",
				Digest:    configDigest,
				Size:      int64(len(configA)), // approximate; size not validated in manifest store
			},
			Layers: layers,
		}
		body := mustJSON(m)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/%s", repo, tag), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", ct)
		req.ContentLength = int64(len(body))
		w := e.do(req)
		if w.Code != http.StatusCreated {
			t.Fatalf("push manifest %s: status %d, body: %s", tag, w.Code, w.Body.String())
		}
		return "sha256:" + sha256hex(body)
	}

	// Manifest A: sharedLayer + uniqueLayer
	digestA := pushManifest("v1", configADigest, []descriptor{
		{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: sharedDigest, Size: int64(len(sharedLayer))},
		{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: uniqueDigest, Size: int64(len(uniqueLayer))},
	})
	// Manifest B: only sharedLayer
	digestB := pushManifest("v2", configBDigest, []descriptor{
		{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: sharedDigest, Size: int64(len(sharedLayer))},
	})

	// Delete manifest A. sharedLayer is still referenced by B → not orphaned.
	// uniqueLayer and configA are only in A → orphaned.
	e.db.omu.Lock()
	e.db.events = nil
	e.db.omu.Unlock()

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/%s", repo, digestA), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("DELETE manifest A: status %d, body: %s", w.Code, w.Body.String())
	}

	e.db.omu.Lock()
	eventsAfterA := append([]capturedEvent(nil), e.db.events...)
	e.db.omu.Unlock()

	// sharedDigest must NOT appear in negative storage-bytes events after deleting A.
	for _, ev := range eventsAfterA {
		if ev.metric == "storage-bytes" && ev.value < 0 && ev.digest == sharedDigest {
			t.Fatalf("DELETE manifest A: shared blob %s incorrectly reported as orphaned", sharedDigest)
		}
	}
	// uniqueLayer and configA MUST appear as orphaned (negative storage-bytes).
	for _, want := range []string{uniqueDigest, configADigest} {
		var found bool
		for _, ev := range eventsAfterA {
			if ev.metric == "storage-bytes" && ev.value < 0 && ev.digest == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("DELETE manifest A: expected orphaned blob %s to emit negative storage-bytes event; events: %+v", want, eventsAfterA)
		}
	}

	// Delete manifest B. sharedLayer is now orphaned.
	e.db.omu.Lock()
	e.db.events = nil
	e.db.omu.Unlock()

	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/%s", repo, digestB), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("DELETE manifest B: status %d, body: %s", w.Code, w.Body.String())
	}

	e.db.omu.Lock()
	eventsAfterB := append([]capturedEvent(nil), e.db.events...)
	e.db.omu.Unlock()

	var foundShared bool
	for _, ev := range eventsAfterB {
		if ev.metric == "storage-bytes" && ev.value < 0 && ev.digest == sharedDigest {
			foundShared = true
			break
		}
	}
	if !foundShared {
		t.Fatalf("DELETE manifest B: shared blob %s should now be orphaned; events: %+v", sharedDigest, eventsAfterB)
	}
}

// TestProbeCache verifies the debounce logic: the first call to shouldUpdate
// for a digest returns true; a second call within 30 s returns false.
func TestProbeCache(t *testing.T) {
	pc := &probeCache{recent: make(map[string]time.Time)}
	const digest = "sha256:" + "a1b2c3"

	if !pc.shouldUpdate(digest) {
		t.Fatal("first shouldUpdate: expected true")
	}
	if pc.shouldUpdate(digest) {
		t.Fatal("second shouldUpdate (within debounce): expected false")
	}
	// A different digest is independent.
	if !pc.shouldUpdate("sha256:other") {
		t.Fatal("different digest: expected true")
	}
}

// TestManifestPushByDigest verifies that PUT /v2/<repo>/manifests/<digest>
// stores the manifest without creating a tag, and that it is retrievable by
// digest but not by a tag reference.
func TestManifestPushByDigest(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/digestpush"

	configDigest := e.pushBlob(t, repo, []byte(`{}`))
	layerDigest := e.pushBlob(t, repo, []byte("digest push layer"))

	manifest := imageManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config: descriptor{
			MediaType: "application/vnd.oci.image.config.v1+json",
			Digest:    configDigest,
			Size:      2,
		},
		Layers: []descriptor{{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    layerDigest,
			Size:      int64(len("digest push layer")),
		}},
	}
	body := mustJSON(manifest)
	digestRef := "sha256:" + sha256hex(body)

	tok := e.token(t, repo, "push", "pull")
	ct := "application/vnd.oci.image.manifest.v1+json"

	// PUT directly to the digest reference (no tag).
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/%s", repo, digestRef), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", ct)
	req.ContentLength = int64(len(body))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PUT manifest by digest: status %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Docker-Content-Digest"); got != digestRef {
		t.Fatalf("Docker-Content-Digest = %q, want %q", got, digestRef)
	}

	// GET by digest must succeed.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/%s", repo, digestRef), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET manifest by digest: status %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), body) {
		t.Fatal("GET manifest by digest: body mismatch")
	}

	// No tag should have been created — tag list must be empty.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/tags/list", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("tags/list: status %d", w.Code)
	}
	var resp tagListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("tags/list: unmarshal: %v", err)
	}
	if len(resp.Tags) != 0 {
		t.Fatalf("tags/list: expected no tags after push-by-digest, got %v", resp.Tags)
	}
}

// TestBlobUploadUnknownUUID verifies that PATCH and PUT against an upload
// session UUID that was never started return 404 BLOB_UPLOAD_UNKNOWN.
func TestBlobUploadUnknownUUID(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/unknownuuid"
	tok := e.token(t, repo, "push")

	fakeUUID := "00000000-0000-0000-0000-000000000000"
	uploadPath := fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, fakeUUID)

	// PATCH against unknown session.
	req := httptest.NewRequest(http.MethodPatch, uploadPath, bytes.NewReader([]byte("data")))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.ContentLength = 4
	w := e.do(req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("PATCH unknown UUID: status %d, want 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "BLOB_UPLOAD_UNKNOWN") {
		t.Fatalf("PATCH unknown UUID: body %q does not contain BLOB_UPLOAD_UNKNOWN", w.Body.String())
	}

	// PUT (finalize) against unknown session.
	digestStr := "sha256:" + strings.Repeat("a", 64)
	req = httptest.NewRequest(http.MethodPut, uploadPath+"?digest="+digestStr, http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("PUT unknown UUID: status %d, want 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "BLOB_UPLOAD_UNKNOWN") {
		t.Fatalf("PUT unknown UUID: body %q does not contain BLOB_UPLOAD_UNKNOWN", w.Body.String())
	}
}

// TestReferrerChain verifies that GET /referrers/<digest> returns only direct
// referrers, not transitive ones.  Given A ← B ← C (C refers to B which refers
// to A), listing referrers of A should include B but not C.
func TestReferrerChain(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/refchain"

	configDigest := e.pushBlob(t, repo, []byte(`{}`))
	layerDigest := e.pushBlob(t, repo, []byte("chain layer"))

	tok := e.token(t, repo, "push", "pull")
	ct := "application/vnd.oci.image.manifest.v1+json"

	pushManifest := func(tag string, subject *descriptor, artifactType string) ([]byte, string) {
		t.Helper()
		m := imageManifest{
			SchemaVersion: 2,
			MediaType:     ct,
			ArtifactType:  artifactType,
			Config: descriptor{
				MediaType: "application/vnd.oci.image.config.v1+json",
				Digest:    configDigest,
				Size:      2,
			},
			Layers: []descriptor{{
				MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
				Digest:    layerDigest,
				Size:      int64(len("chain layer")),
			}},
			Subject: subject,
		}
		body := mustJSON(m)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/%s", repo, tag), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", ct)
		req.ContentLength = int64(len(body))
		w := e.do(req)
		if w.Code != http.StatusCreated {
			t.Fatalf("push manifest %s: status %d, body: %s", tag, w.Code, w.Body.String())
		}
		return body, "sha256:" + sha256hex(body)
	}

	// A — the root subject.
	aBody, aDigest := pushManifest("a", nil, "")

	// B — refers to A.
	_, bDigest := pushManifest("b", &descriptor{
		MediaType: ct,
		Digest:    aDigest,
		Size:      int64(len(aBody)),
	}, "application/vnd.example.attestation")

	// C — refers to B (not A).
	bBody := mustJSON(imageManifest{
		SchemaVersion: 2, MediaType: ct, ArtifactType: "application/vnd.example.attestation",
		Config:  descriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: configDigest, Size: 2},
		Layers:  []descriptor{{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: layerDigest, Size: int64(len("chain layer"))}},
		Subject: &descriptor{MediaType: ct, Digest: aDigest, Size: int64(len(aBody))},
	})
	pushManifest("c", &descriptor{
		MediaType: ct,
		Digest:    bDigest,
		Size:      int64(len(bBody)),
	}, "application/vnd.example.provenance")

	// GET referrers of A — should contain B only, not C.
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/referrers/%s", repo, aDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET referrers of A: status %d", w.Code)
	}
	var idx imageIndex
	if err := json.Unmarshal(w.Body.Bytes(), &idx); err != nil {
		t.Fatalf("GET referrers of A: unmarshal: %v", err)
	}
	if len(idx.Manifests) != 1 {
		t.Fatalf("GET referrers of A: got %d manifests, want 1 (B only); digests: %v",
			len(idx.Manifests), func() []string {
				out := make([]string, len(idx.Manifests))
				for i, m := range idx.Manifests {
					out[i] = m.Digest
				}
				return out
			}())
	}
	if idx.Manifests[0].Digest != bDigest {
		t.Fatalf("GET referrers of A: got %s, want B (%s)", idx.Manifests[0].Digest, bDigest)
	}
}

// TestManifestDeleteNotFound verifies that deleting a manifest digest that was
// never pushed returns 404, not an internal error.
func TestManifestDeleteNotFound(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/dne"
	tok := e.token(t, repo, "push")

	ghost := "sha256:" + strings.Repeat("0", 64)
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/%s", repo, ghost), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("DELETE non-existent manifest: status %d, want 404", w.Code)
	}
}

// TestTagReassignment verifies that pushing a different manifest to an existing
// tag updates the tag to point to the new manifest, while the old manifest
// remains reachable by digest.
func TestTagReassignment(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/reassign"
	tok := e.token(t, repo, "push", "pull")
	ct := "application/vnd.oci.image.manifest.v1+json"

	configDigest := e.pushBlob(t, repo, []byte(`{}`))
	layerA := e.pushBlob(t, repo, []byte("layer for manifest A"))
	layerB := e.pushBlob(t, repo, []byte("layer for manifest B"))

	pushManifest := func(layerDigest string) ([]byte, string) {
		t.Helper()
		m := imageManifest{
			SchemaVersion: 2,
			MediaType:     ct,
			Config:        descriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: configDigest, Size: 2},
			Layers:        []descriptor{{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: layerDigest, Size: 20}},
		}
		body := mustJSON(m)
		return body, "sha256:" + sha256hex(body)
	}

	bodyA, digestA := pushManifest(layerA)
	bodyB, digestB := pushManifest(layerB)

	// Push A to "latest".
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/latest", repo), bytes.NewReader(bodyA))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", ct)
	req.ContentLength = int64(len(bodyA))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push A: status %d, body: %s", w.Code, w.Body.String())
	}

	// Reassign "latest" to B.
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/latest", repo), bytes.NewReader(bodyB))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", ct)
	req.ContentLength = int64(len(bodyB))
	w = e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push B: status %d, body: %s", w.Code, w.Body.String())
	}

	// "latest" must now resolve to B.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/latest", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET latest: status %d", w.Code)
	}
	if got := w.Header().Get("Docker-Content-Digest"); got != digestB {
		t.Fatalf("GET latest: digest = %q, want B (%s)", got, digestB)
	}

	// A must still be reachable by its digest.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/%s", repo, digestA), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET A by digest: status %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), bodyA) {
		t.Fatal("GET A by digest: body mismatch")
	}
}

// TestManifestDoubleDelete verifies that deleting the same manifest digest
// twice returns 202 on the first delete and 404 on the second (idempotent).
func TestManifestDoubleDelete(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/doubledelete"
	tok := e.token(t, repo, "push")
	ct := "application/vnd.oci.image.manifest.v1+json"

	configDigest := e.pushBlob(t, repo, []byte(`{}`))
	layerDigest := e.pushBlob(t, repo, []byte("layer"))

	m := imageManifest{
		SchemaVersion: 2, MediaType: ct,
		Config: descriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: configDigest, Size: 2},
		Layers: []descriptor{{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: layerDigest, Size: 5}},
	}
	body := mustJSON(m)
	digest := "sha256:" + sha256hex(body)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/v1.0", repo), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", ct)
	req.ContentLength = int64(len(body))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push: status %d", w.Code)
	}

	del := func() int {
		t.Helper()
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/v2/%s/manifests/%s", repo, digest), nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		return e.do(req).Code
	}

	if code := del(); code != http.StatusAccepted {
		t.Fatalf("first DELETE: status %d, want 202", code)
	}
	if code := del(); code != http.StatusNotFound {
		t.Fatalf("second DELETE: status %d, want 404", code)
	}
}

// TestManifestContentTypeFromHeader verifies that the Content-Type header on
// PUT governs the stored media type, regardless of the "mediaType" field inside
// the JSON body.  On GET the server must echo back the header value, not the
// JSON body field.
func TestManifestContentTypeFromHeader(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/ctrepo"
	tok := e.token(t, repo, "push", "pull")

	configDigest := e.pushBlob(t, repo, []byte(`{}`))
	layerDigest := e.pushBlob(t, repo, []byte("ct layer"))

	// JSON body declares OCI media type, but header says Docker schema2.
	dockerCT := "application/vnd.docker.distribution.manifest.v2+json"
	m := imageManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json", // intentionally mismatched
		Config:        descriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: configDigest, Size: 2},
		Layers:        []descriptor{{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: layerDigest, Size: 8}},
	}
	body := mustJSON(m)
	digest := "sha256:" + sha256hex(body)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/v1.0", repo), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", dockerCT)
	req.ContentLength = int64(len(body))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PUT manifest: status %d, body: %s", w.Code, w.Body.String())
	}

	// GET by digest — Content-Type in response must match the PUT header, not JSON field.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/%s", repo, digest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET manifest: status %d", w.Code)
	}
	got := w.Header().Get("Content-Type")
	if got != dockerCT {
		t.Fatalf("Content-Type = %q, want %q (header must take precedence over JSON mediaType field)", got, dockerCT)
	}
}

// TestOCIIndexNoMediaTypeField verifies that an OCI image index whose JSON body
// omits the "mediaType" field is accepted, as permitted by the OCI spec.
// The Content-Type header is the authoritative type signal.
func TestOCIIndexNoMediaTypeField(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/nomediatype"
	tok := e.token(t, repo, "push", "pull")

	// Push a child manifest first.
	configDigest := e.pushBlob(t, repo, []byte(`{}`))
	layerDigest := e.pushBlob(t, repo, []byte("nomediatype layer"))
	child := imageManifest{
		SchemaVersion: 2,
		MediaType:     "application/vnd.oci.image.manifest.v1+json",
		Config:        descriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: configDigest, Size: 2},
		Layers:        []descriptor{{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: layerDigest, Size: 17}},
	}
	childBody := mustJSON(child)
	childDigest := "sha256:" + sha256hex(childBody)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/child", repo), bytes.NewReader(childBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
	req.ContentLength = int64(len(childBody))
	w := e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("push child: status %d, body: %s", w.Code, w.Body.String())
	}

	// Build an index JSON without the "mediaType" field by marshalling a struct
	// that omits it, then push with the OCI index Content-Type.
	type bareIndex struct {
		SchemaVersion int          `json:"schemaVersion"`
		Manifests     []descriptor `json:"manifests"`
	}
	indexBody := mustJSON(bareIndex{
		SchemaVersion: 2,
		Manifests: []descriptor{{
			MediaType: "application/vnd.oci.image.manifest.v1+json",
			Digest:    childDigest,
			Size:      int64(len(childBody)),
		}},
	})
	indexCT := "application/vnd.oci.image.index.v1+json"

	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/index", repo), bytes.NewReader(indexBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", indexCT)
	req.ContentLength = int64(len(indexBody))
	w = e.do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PUT index without mediaType field: status %d, body: %s", w.Code, w.Body.String())
	}

	// GET must return the index with the correct Content-Type from the PUT header.
	indexDigest := "sha256:" + sha256hex(indexBody)
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/manifests/%s", repo, indexDigest), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w = e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET index: status %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != indexCT {
		t.Fatalf("GET index: Content-Type = %q, want %q", got, indexCT)
	}
}

// TestTagListEmptyArray verifies that a repository with no tags returns an
// empty JSON array ("tags":[]) rather than a null field, as required by the
// OCI Distribution Spec.
func TestTagListEmptyArray(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/notags"
	tok := e.token(t, repo, "pull")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v2/%s/tags/list", repo), nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := e.do(req)
	if w.Code != http.StatusOK {
		t.Fatalf("tags/list: status %d", w.Code)
	}

	// Unmarshal as raw map so we can distinguish [] from null.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tagsField, ok := raw["tags"]
	if !ok {
		t.Fatal("tags/list: response missing 'tags' field")
	}
	if string(tagsField) == "null" {
		t.Fatal("tags/list: 'tags' field is null, want []")
	}
	var tags []string
	if err := json.Unmarshal(tagsField, &tags); err != nil {
		t.Fatalf("tags/list: tags field is not an array: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("tags/list: expected empty array, got %v", tags)
	}
}

// TestManifestPushParallelIdempotent verifies that two goroutines pushing the
// same manifest digest to the same tag simultaneously both receive 201 with the
// same Docker-Content-Digest.  Tests for TOCTOU bugs in the upsert path.
func TestManifestPushParallelIdempotent(t *testing.T) {
	e := newTestRegistryEnv(t, "alpha")
	repo := "alpha/parallel"
	tok := e.token(t, repo, "push", "pull")
	ct := "application/vnd.oci.image.manifest.v1+json"

	configDigest := e.pushBlob(t, repo, []byte(`{}`))
	layerDigest := e.pushBlob(t, repo, []byte("parallel layer"))

	m := imageManifest{
		SchemaVersion: 2, MediaType: ct,
		Config: descriptor{MediaType: "application/vnd.oci.image.config.v1+json", Digest: configDigest, Size: 2},
		Layers: []descriptor{{MediaType: "application/vnd.oci.image.layer.v1.tar+gzip", Digest: layerDigest, Size: 14}},
	}
	body := mustJSON(m)
	wantDigest := "sha256:" + sha256hex(body)

	const workers = 8
	type result struct {
		code   int
		digest string
	}
	results := make([]result, workers)
	var wg sync.WaitGroup
	// ready gates all goroutines to start at the same time.
	ready := make(chan struct{})

	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-ready
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/v2/%s/manifests/latest", repo), bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+tok)
			req.Header.Set("Content-Type", ct)
			req.ContentLength = int64(len(body))
			w := e.do(req)
			results[idx] = result{code: w.Code, digest: w.Header().Get("Docker-Content-Digest")}
		}(i)
	}
	close(ready)
	wg.Wait()

	for i, r := range results {
		if r.code != http.StatusCreated {
			t.Errorf("worker %d: status %d, want 201", i, r.code)
		}
		if r.digest != wantDigest {
			t.Errorf("worker %d: digest = %q, want %q", i, r.digest, wantDigest)
		}
	}
}
