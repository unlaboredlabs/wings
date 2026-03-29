package environment

import (
	"context"
	"sync"

	"emperror.dev/errors"
	"github.com/docker/docker/client"

	"github.com/Minenetpro/pelican-wings/config"
)

var (
	_conce  sync.Once
	_client *client.Client
)

// Docker returns a docker client to be used throughout the codebase. Once a
// client has been created it will be returned for all subsequent calls to this
// function.
func Docker() (*client.Client, error) {
	var err error
	_conce.Do(func() {
		_client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	})
	return _client, errors.Wrap(err, "environment/docker: could not create client")
}

// ConfigureDocker validates the docker daemon capabilities required for the
// secure shared-node runtime model.
func ConfigureDocker(ctx context.Context) error {
	cli, err := Docker()
	if err != nil {
		return err
	}

	info, err := cli.Info(ctx)
	if err != nil {
		return errors.Wrap(err, "environment/docker: failed to inspect docker daemon capabilities")
	}

	runtimeName := config.Get().Docker.Runtime
	if runtimeName == "" {
		return errors.New("environment/docker: docker.runtime must be configured")
	}

	if _, ok := info.Runtimes[runtimeName]; !ok {
		return errors.Errorf("environment/docker: required runtime %q is not available on this node", runtimeName)
	}

	return nil
}
