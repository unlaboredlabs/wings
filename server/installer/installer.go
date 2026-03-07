package installer

import (
	"context"
	"emperror.dev/errors"
	"github.com/asaskevich/govalidator"

	"github.com/Minenetpro/pelican-wings/remote"
	"github.com/Minenetpro/pelican-wings/server"
)

type Installer struct {
	server            *server.Server
	StartOnCompletion bool
	SkipInstall       bool
}

type ServerDetails struct {
	UUID                 string                       `json:"uuid"`
	StartOnCompletion    bool                         `json:"start_on_completion"`
	SkipInstall          bool                         `json:"skip_install"`
	Settings             []byte                       `json:"settings,omitempty"`
	ProcessConfiguration *remote.ProcessConfiguration `json:"process_configuration,omitempty"`
	InstallationScript   *remote.InstallationScript   `json:"installation_script,omitempty"`
}

// New validates the received data to ensure that all the required fields
// have been passed along in the request. This should be manually run before
// calling Execute().
func New(ctx context.Context, manager *server.Manager, details ServerDetails) (*Installer, error) {
	if !govalidator.IsUUIDv4(details.UUID) {
		return nil, NewValidationError("uuid provided was not in a valid format")
	}

	var c remote.ServerConfigurationResponse
	if len(details.Settings) > 0 && details.ProcessConfiguration != nil {
		if localStore, ok := manager.Client().(remote.LocalServerStore); ok {
			if err := localStore.UpsertServer(ctx, remote.LocalServerDefinition{
				UUID: details.UUID,
				Configuration: remote.ServerConfigurationResponse{
					Settings:             details.Settings,
					ProcessConfiguration: details.ProcessConfiguration,
				},
				InstallationScript: details.InstallationScript,
			}); err != nil {
				return nil, errors.WrapIf(err, "installer: could not persist local server definition")
			}
		}
		c = remote.ServerConfigurationResponse{
			Settings:             details.Settings,
			ProcessConfiguration: details.ProcessConfiguration,
		}
	} else {
		var err error
		c, err = manager.Client().GetServerConfiguration(ctx, details.UUID)
		if err != nil {
			if !remote.IsRequestError(err) {
				return nil, errors.WithStackIf(err)
			}
			return nil, errors.WrapIf(err, "installer: could not get server configuration from remote API")
		}
	}

	// Create a new server instance using the configuration we wrote to the disk
	// so that everything gets instantiated correctly on the struct.
	s, err := manager.InitServer(c)
	if err != nil {
		return nil, errors.WrapIf(err, "installer: could not init server instance")
	}
	i := Installer{
		server:            s,
		StartOnCompletion: details.StartOnCompletion,
		SkipInstall:       details.SkipInstall,
	}
	return &i, nil
}

// Server returns the server instance.
func (i *Installer) Server() *server.Server {
	return i.server
}
