package server

import (
	"reflect"
	"testing"
)

func TestExtractManifestReferencesFromImageManifest(t *testing.T) {
	manifest := imageManifest{
		SchemaVersion: 2,
		Config: descriptor{
			Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Layers: []descriptor{
			{Digest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
			{Digest: ""},
			{Digest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
		},
	}

	blobDigests, childManifestDigests := extractManifestReferences(manifest)
	if !reflect.DeepEqual(blobDigests, []string{
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}) {
		t.Fatalf("blob digests = %#v", blobDigests)
	}
	if len(childManifestDigests) != 0 {
		t.Fatalf("child manifest digests = %#v, want none", childManifestDigests)
	}
}

func TestExtractManifestReferencesFromIndexManifest(t *testing.T) {
	manifest := imageManifest{
		SchemaVersion: 2,
		Manifests: []descriptor{
			{Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111"},
			{Digest: ""},
			{Digest: "sha256:2222222222222222222222222222222222222222222222222222222222222222"},
		},
	}

	blobDigests, childManifestDigests := extractManifestReferences(manifest)
	if len(blobDigests) != 0 {
		t.Fatalf("blob digests = %#v, want none", blobDigests)
	}
	if !reflect.DeepEqual(childManifestDigests, []string{
		"sha256:1111111111111111111111111111111111111111111111111111111111111111",
		"sha256:2222222222222222222222222222222222222222222222222222222222222222",
	}) {
		t.Fatalf("child manifest digests = %#v", childManifestDigests)
	}
}

func TestExtractManifestReferencesEmpty(t *testing.T) {
	blobDigests, childManifestDigests := extractManifestReferences(imageManifest{SchemaVersion: 2})
	if len(blobDigests) != 0 || len(childManifestDigests) != 0 {
		t.Fatalf("got blob=%#v child=%#v, want none", blobDigests, childManifestDigests)
	}
}
