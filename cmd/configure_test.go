package cmd

import "testing"

func TestNewLocalConfigurationGeneratesCredentials(t *testing.T) {
	t.Parallel()

	cfg, generatedTokenID, generatedToken, err := newLocalConfiguration(
		"/tmp/pelican-config.yml",
		"",
		"",
		"https://control.example.com",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !generatedTokenID {
		t.Fatal("expected token id to be generated")
	}
	if !generatedToken {
		t.Fatal("expected token to be generated")
	}
	if cfg.Uuid == "" {
		t.Fatal("expected node uuid to be generated")
	}
	if cfg.AuthenticationTokenId == "" {
		t.Fatal("expected authentication token id to be set")
	}
	if cfg.AuthenticationToken == "" {
		t.Fatal("expected authentication token to be set")
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://control.example.com" {
		t.Fatalf("expected allowed origin to be set, got %#v", cfg.AllowedOrigins)
	}
}

func TestNewLocalConfigurationUsesProvidedCredentials(t *testing.T) {
	t.Parallel()

	cfg, generatedTokenID, generatedToken, err := newLocalConfiguration(
		"/tmp/pelican-config.yml",
		"node-1",
		"secret-token",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if generatedTokenID {
		t.Fatal("did not expect token id generation")
	}
	if generatedToken {
		t.Fatal("did not expect token generation")
	}
	if cfg.AuthenticationTokenId != "node-1" {
		t.Fatalf("expected provided token id to be preserved, got %q", cfg.AuthenticationTokenId)
	}
	if cfg.AuthenticationToken != "secret-token" {
		t.Fatalf("expected provided token to be preserved, got %q", cfg.AuthenticationToken)
	}
}

func TestValidateAllowedOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		origin  string
		wantErr bool
	}{
		{name: "blank is allowed", origin: ""},
		{name: "valid https origin", origin: "https://panel.example.com"},
		{name: "valid http origin", origin: "http://127.0.0.1:3000"},
		{name: "rejects path", origin: "https://panel.example.com/app", wantErr: true},
		{name: "rejects invalid value", origin: "not-a-url", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAllowedOrigin(tt.origin)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
