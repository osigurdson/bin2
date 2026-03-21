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
