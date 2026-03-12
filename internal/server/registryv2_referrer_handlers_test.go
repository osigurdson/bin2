package server

import (
	"encoding/json"
	"testing"

	"bin2.io/internal/db"
)

func TestBuildReferrerDescriptors(t *testing.T) {
	subjectDigest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	records := []db.RepositoryManifestRecord{
		mustReferrerRecord(t, db.RepositoryManifestRecord{
			Digest:      "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			ContentType: defaultManifestContentType,
			Size:        321,
		}, imageManifest{
			SchemaVersion: 2,
			MediaType:     defaultManifestContentType,
			Config: descriptor{
				MediaType: "application/vnd.example.config-a",
				Digest:    "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			},
			Layers: []descriptor{
				{Digest: "sha256:2111111111111111111111111111111111111111111111111111111111111111"},
			},
			Subject: &descriptor{Digest: subjectDigest},
			Annotations: map[string]string{
				"org.opencontainers.test": "config-artifact",
			},
		}),
		mustReferrerRecord(t, db.RepositoryManifestRecord{
			Digest:      "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			ContentType: defaultIndexContentType,
			Size:        654,
		}, imageIndex{
			SchemaVersion: 2,
			MediaType:     defaultIndexContentType,
			ArtifactType:  "application/vnd.example.index",
			Manifests: []descriptor{
				{Digest: "sha256:3111111111111111111111111111111111111111111111111111111111111111"},
			},
			Subject: &descriptor{Digest: subjectDigest},
			Annotations: map[string]string{
				"org.opencontainers.test": "index-artifact",
			},
		}),
		mustReferrerRecord(t, db.RepositoryManifestRecord{
			Digest:      "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			ContentType: defaultManifestContentType,
			Size:        111,
		}, imageManifest{
			SchemaVersion: 2,
			MediaType:     defaultManifestContentType,
			ArtifactType:  "application/vnd.example.other",
			Config: descriptor{
				Digest: "sha256:4111111111111111111111111111111111111111111111111111111111111111",
			},
			Layers: []descriptor{
				{Digest: "sha256:5111111111111111111111111111111111111111111111111111111111111111"},
			},
			Subject: &descriptor{Digest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
		}),
	}

	descriptors, err := buildReferrerDescriptors(records, subjectDigest, "")
	if err != nil {
		t.Fatalf("buildReferrerDescriptors: %v", err)
	}
	if len(descriptors) != 2 {
		t.Fatalf("len(descriptors) = %d, want 2", len(descriptors))
	}
	if descriptors[0].Digest != records[0].Digest {
		t.Fatalf("first digest = %q, want %q", descriptors[0].Digest, records[0].Digest)
	}
	if descriptors[0].ArtifactType != "application/vnd.example.config-a" {
		t.Fatalf("first artifact type = %q", descriptors[0].ArtifactType)
	}
	if descriptors[0].Annotations["org.opencontainers.test"] != "config-artifact" {
		t.Fatalf("first annotations = %#v", descriptors[0].Annotations)
	}
	if descriptors[1].Digest != records[1].Digest {
		t.Fatalf("second digest = %q, want %q", descriptors[1].Digest, records[1].Digest)
	}
	if descriptors[1].ArtifactType != "application/vnd.example.index" {
		t.Fatalf("second artifact type = %q", descriptors[1].ArtifactType)
	}
}

func TestBuildReferrerDescriptorsArtifactTypeFilter(t *testing.T) {
	subjectDigest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	records := []db.RepositoryManifestRecord{
		mustReferrerRecord(t, db.RepositoryManifestRecord{
			Digest:      "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			ContentType: defaultManifestContentType,
			Size:        321,
		}, imageManifest{
			SchemaVersion: 2,
			MediaType:     defaultManifestContentType,
			Config: descriptor{
				MediaType: "application/vnd.example.keep",
				Digest:    "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			},
			Layers:  []descriptor{{Digest: "sha256:2111111111111111111111111111111111111111111111111111111111111111"}},
			Subject: &descriptor{Digest: subjectDigest},
		}),
		mustReferrerRecord(t, db.RepositoryManifestRecord{
			Digest:      "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			ContentType: defaultManifestContentType,
			Size:        123,
		}, imageManifest{
			SchemaVersion: 2,
			MediaType:     defaultManifestContentType,
			ArtifactType:  "application/vnd.example.skip",
			Config: descriptor{
				Digest: "sha256:3111111111111111111111111111111111111111111111111111111111111111",
			},
			Layers:  []descriptor{{Digest: "sha256:4111111111111111111111111111111111111111111111111111111111111111"}},
			Subject: &descriptor{Digest: subjectDigest},
		}),
	}

	descriptors, err := buildReferrerDescriptors(records, subjectDigest, "application/vnd.example.keep")
	if err != nil {
		t.Fatalf("buildReferrerDescriptors: %v", err)
	}
	if len(descriptors) != 1 {
		t.Fatalf("len(descriptors) = %d, want 1", len(descriptors))
	}
	if descriptors[0].Digest != records[0].Digest {
		t.Fatalf("digest = %q, want %q", descriptors[0].Digest, records[0].Digest)
	}
}

func mustReferrerRecord(t *testing.T, record db.RepositoryManifestRecord, body any) db.RepositoryManifestRecord {
	t.Helper()

	manifestBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	record.Body = manifestBody
	if record.Size == 0 {
		record.Size = int64(len(manifestBody))
	}
	return record
}
