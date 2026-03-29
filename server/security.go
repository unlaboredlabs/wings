package server

import (
	"fmt"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

func validateSecureConfiguration(c *Configuration) error {
	if err := validateSecureImageReference(c.Container.Image); err != nil {
		return err
	}

	if len(c.Mounts) > 0 {
		return fmt.Errorf("custom server mounts are not supported in secure multi-tenant mode")
	}

	if c.Allocations.ForceOutgoingIP {
		return fmt.Errorf("force_outgoing_ip is not supported in secure multi-tenant mode")
	}

	return nil
}

func validateSecureImageReference(image string) error {
	if !isDigestPinnedImage(image) {
		return fmt.Errorf("image %q must be pinned by digest", image)
	}

	return nil
}

func isDigestPinnedImage(image string) bool {
	name, digestValue, ok := strings.Cut(image, "@")
	if !ok || name == "" || digestValue == "" {
		return false
	}

	_, err := digest.Parse(digestValue)
	return err == nil
}
