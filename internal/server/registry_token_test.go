package server

import "testing"

func TestGrantRegistryTokenScopes(t *testing.T) {
	requested := []registryTokenAccess{
		{Type: "repository", Name: "alpha/app", Actions: []string{"pull", "push"}},
		{Type: "repository", Name: "alpha/app", Actions: []string{"pull"}},
		{Type: "repository", Name: "beta/app", Actions: []string{"pull", "push"}},
		{Type: "registry", Name: "catalog", Actions: []string{"*"}},
	}

	granted := grantRegistryTokenScopes("alpha", requested)
	if len(granted) != 1 {
		t.Fatalf("granted len = %d, want 1", len(granted))
	}
	if granted[0].Type != "repository" || granted[0].Name != "alpha/app" {
		t.Fatalf("granted[0] = %#v", granted[0])
	}
	if len(granted[0].Actions) != 2 || granted[0].Actions[0] != "pull" || granted[0].Actions[1] != "push" {
		t.Fatalf("actions = %#v", granted[0].Actions)
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
