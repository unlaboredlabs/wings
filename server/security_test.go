package server

import (
	"testing"

	"github.com/Minenetpro/pelican-wings/environment"
)

func secureTestConfig(image string) Configuration {
	var c Configuration
	c.Container.Image = image
	return c
}

func TestValidateSecureConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  Configuration
		wantErr bool
	}{
		{
			name:   "accepts digest pinned image",
			config: secureTestConfig("ghcr.io/pelican-dev/example@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		},
		{
			name:    "rejects tag image",
			config:  secureTestConfig("ghcr.io/pelican-dev/example:latest"),
			wantErr: true,
		},
		{
			name: "rejects custom mounts",
			config: func() Configuration {
				c := secureTestConfig("ghcr.io/pelican-dev/example@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
				c.Mounts = []Mount{{Source: "/tmp/source", Target: "/tmp/target"}}
				return c
			}(),
			wantErr: true,
		},
		{
			name: "rejects forced outgoing ip",
			config: func() Configuration {
				c := secureTestConfig("ghcr.io/pelican-dev/example@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
				c.Allocations = environment.Allocations{ForceOutgoingIP: true}
				return c
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateSecureConfiguration(&tt.config)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateSecureImageReference(t *testing.T) {
	t.Parallel()

	if err := validateSecureImageReference("ghcr.io/pelican-dev/example@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err != nil {
		t.Fatalf("expected digest image to pass validation: %v", err)
	}

	if err := validateSecureImageReference("ghcr.io/pelican-dev/example:latest"); err == nil {
		t.Fatal("expected tag image to fail validation")
	}
}
