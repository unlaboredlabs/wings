package middleware

import "testing"

func TestResolveAllowedOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		location      string
		origins       []string
		requestOrigin string
		want          string
	}{
		{
			name:          "uses matching legacy location",
			location:      "https://panel.example.com",
			requestOrigin: "https://panel.example.com",
			want:          "https://panel.example.com",
		},
		{
			name:          "uses explicit allowed origin when location empty",
			origins:       []string{"https://control.example.com"},
			requestOrigin: "https://control.example.com",
			want:          "https://control.example.com",
		},
		{
			name:          "wildcard reflects request origin",
			origins:       []string{"*"},
			requestOrigin: "https://control.example.com",
			want:          "https://control.example.com",
		},
		{
			name:          "returns empty when nothing is configured",
			requestOrigin: "https://control.example.com",
			want:          "",
		},
		{
			name:     "falls back to location when request origin missing",
			location: "https://panel.example.com",
			want:     "https://panel.example.com",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveAllowedOrigin(tt.location, tt.origins, tt.requestOrigin)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
