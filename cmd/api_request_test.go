package cmd

import "testing"

func TestResolveAPITokenPrefersFlagOverEnv(t *testing.T) {
	t.Setenv("XRMCP_API_TOKEN", "env-token")

	if got := resolveAPIToken("flag-token"); got != "flag-token" {
		t.Fatalf("expected flag token, got %q", got)
	}
}

func TestResolveAPITokenFallsBackToEnv(t *testing.T) {
	t.Setenv("XRMCP_API_TOKEN", "env-token")

	if got := resolveAPIToken(""); got != "env-token" {
		t.Fatalf("expected env token, got %q", got)
	}
}

func TestNewAPIRequestAddsAuthorizationHeader(t *testing.T) {
	req, err := newAPIRequest("GET", "http://localhost:7373/tools/list-installed", "secret-token", nil)
	if err != nil {
		t.Fatalf("newAPIRequest: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer secret-token" {
		t.Fatalf("expected Authorization header, got %q", got)
	}
}

func TestNewAPIRequestOmitsAuthorizationHeaderWithoutToken(t *testing.T) {
	req, err := newAPIRequest("GET", "http://localhost:7373/tools/list-installed", "", nil)
	if err != nil {
		t.Fatalf("newAPIRequest: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected no Authorization header, got %q", got)
	}
}
