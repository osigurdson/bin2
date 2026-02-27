package server

import (
	"testing"

	"bin2.io/internal/db"
	"github.com/google/uuid"
)

func TestGrantRegistryTokenScopes(t *testing.T) {
	registryID := uuid.New()
	requested := []registryTokenAccess{
		{Type: "repository", Name: "alpha/app", Actions: []string{"pull", "push"}},
		{Type: "repository", Name: "alpha/app", Actions: []string{"pull"}},
		{Type: "repository", Name: "alpha/worker", Actions: []string{"pull", "push"}},
		{Type: "registry", Name: "catalog", Actions: []string{"*"}},
	}
	apiScopes := []db.APIKeyScope{
		{
			RegistryID: registryID,
			Permission: db.APIKeyPermissionRead,
		},
	}

	granted := grantRegistryTokenScopes(registryID, "alpha", apiScopes, requested)
	if len(granted) != 2 {
		t.Fatalf("granted len = %d, want 2", len(granted))
	}
	if granted[0].Type != "repository" || granted[0].Name != "alpha/app" {
		t.Fatalf("granted[0] = %#v", granted[0])
	}
	if len(granted[0].Actions) != 1 || granted[0].Actions[0] != "pull" {
		t.Fatalf("actions = %#v", granted[0].Actions)
	}
	if granted[1].Name != "alpha/worker" || len(granted[1].Actions) != 1 || granted[1].Actions[0] != "pull" {
		t.Fatalf("granted[1] = %#v", granted[1])
	}
}

func TestRequiredRegistryScope(t *testing.T) {
	tests := []struct {
		path      string
		method    string
		wantScope string
	}{
		{path: "alpha/app/blobs/uploads/", method: "POST", wantScope: "repository:alpha/app:push"},
		{path: "alpha/app/blobs/uploads/123", method: "PATCH", wantScope: "repository:alpha/app:push"},
		{path: "alpha/app/blobs/uploads/123", method: "PUT", wantScope: "repository:alpha/app:push"},
		{path: "alpha/app/blobs/sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", method: "GET", wantScope: "repository:alpha/app:pull"},
		{path: "alpha/app/manifests/latest", method: "GET", wantScope: "repository:alpha/app:pull"},
		{path: "alpha/app/manifests/latest", method: "PUT", wantScope: "repository:alpha/app:push"},
		{path: "", method: "GET", wantScope: ""},
		{path: "token", method: "GET", wantScope: ""},
	}

	for _, tt := range tests {
		_, got := requiredRegistryScope(tt.path, tt.method)
		if got != tt.wantScope {
			t.Fatalf("path=%q method=%q scope=%q, want %q", tt.path, tt.method, got, tt.wantScope)
		}
	}
}

func TestRegistryTokenAllows(t *testing.T) {
	access := []registryTokenAccess{{
		Type:    "repository",
		Name:    "alpha/app",
		Actions: []string{"pull", "push"},
	}}

	if !registryTokenAllows(access, "alpha/app", "pull") {
		t.Fatalf("expected pull allowed")
	}
	if !registryTokenAllows(access, "alpha/app", "push") {
		t.Fatalf("expected push allowed")
	}
	if registryTokenAllows(access, "alpha/other", "pull") {
		t.Fatalf("expected other repo denied")
	}
}

func TestGrantRegistryTokenScopesRespectsRepoScope(t *testing.T) {
	registryID := uuid.New()
	repo := "app"
	apiScopes := []db.APIKeyScope{{
		RegistryID: registryID,
		Repository: &repo,
		Permission: db.APIKeyPermissionWrite,
	}}
	requested := []registryTokenAccess{
		{Type: "repository", Name: "alpha/app", Actions: []string{"pull", "push"}},
		{Type: "repository", Name: "alpha/other", Actions: []string{"pull", "push"}},
	}

	granted := grantRegistryTokenScopes(registryID, "alpha", apiScopes, requested)
	if len(granted) != 1 {
		t.Fatalf("granted len = %d, want 1", len(granted))
	}
	if granted[0].Name != "alpha/app" {
		t.Fatalf("granted[0].Name = %q", granted[0].Name)
	}
	if len(granted[0].Actions) != 2 || granted[0].Actions[0] != "pull" || granted[0].Actions[1] != "push" {
		t.Fatalf("actions = %#v", granted[0].Actions)
	}
}
