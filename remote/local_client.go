package remote

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/Minenetpro/pelican-wings/config"
	"github.com/Minenetpro/pelican-wings/internal/models"
	"github.com/apex/log"
	"github.com/gbrlsnchs/jwt/v3"
	"github.com/goccy/go-json"
)

const localStoreFilename = "servers.json"

type LocalServerDefinition struct {
	UUID               string                      `json:"uuid"`
	Configuration      ServerConfigurationResponse `json:"configuration"`
	InstallationScript *InstallationScript         `json:"installation_script,omitempty"`
}

type LocalServerStore interface {
	UpsertServer(ctx context.Context, definition LocalServerDefinition) error
	DeleteServer(ctx context.Context, uuid string) error
	GetLocalServer(ctx context.Context, uuid string) (LocalServerDefinition, error)
}

type localClient struct {
	mu      sync.RWMutex
	path    string
	servers map[string]LocalServerDefinition
}

type localStoreDocument struct {
	Servers map[string]LocalServerDefinition `json:"servers"`
}

type localSftpToken struct {
	jwt.Payload
	Server      string   `json:"server"`
	User        string   `json:"user"`
	Username    string   `json:"username"`
	Permissions []string `json:"permissions"`
}

func (t *localSftpToken) GetPayload() *jwt.Payload {
	return &t.Payload
}

func NewLocal(root string) (Client, error) {
	c := &localClient{
		path:    filepath.Join(root, localStoreFilename),
		servers: map[string]LocalServerDefinition{},
	}
	if err := c.load(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *localClient) load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return errors.Wrap(err, "remote/local: failed to create local server store directory")
	}

	data, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "remote/local: failed to read local server store")
	}
	if len(data) == 0 {
		return nil
	}

	var doc localStoreDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return errors.Wrap(err, "remote/local: failed to parse local server store")
	}

	if doc.Servers != nil {
		c.servers = doc.Servers
	}
	return nil
}

func (c *localClient) persistLocked() error {
	doc := localStoreDocument{Servers: c.servers}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return errors.Wrap(err, "remote/local: failed to marshal local server store")
	}
	if err := os.WriteFile(c.path, data, 0o600); err != nil {
		return errors.Wrap(err, "remote/local: failed to persist local server store")
	}
	return nil
}

func (c *localClient) UpsertServer(_ context.Context, definition LocalServerDefinition) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if definition.UUID == "" {
		return errors.New("remote/local: missing server uuid")
	}
	definition.Configuration.ProcessConfiguration = cloneProcessConfiguration(definition.Configuration.ProcessConfiguration)
	definition.Configuration.Settings = append([]byte(nil), definition.Configuration.Settings...)
	definition.InstallationScript = cloneInstallationScript(definition.InstallationScript)
	c.servers[definition.UUID] = definition
	return c.persistLocked()
}

func (c *localClient) DeleteServer(_ context.Context, uuid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.servers, uuid)
	return c.persistLocked()
}

func (c *localClient) GetLocalServer(_ context.Context, uuid string) (LocalServerDefinition, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	server, ok := c.servers[uuid]
	if !ok {
		return LocalServerDefinition{}, &RequestError{
			response: &http.Response{StatusCode: http.StatusNotFound},
			Code:     "NotFound",
			Detail:   "server does not exist in local store",
		}
	}
	return cloneLocalServerDefinition(server), nil
}

func (c *localClient) GetServers(_ context.Context, _ int) ([]RawServerData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	uuids := make([]string, 0, len(c.servers))
	for uuid := range c.servers {
		uuids = append(uuids, uuid)
	}
	sort.Strings(uuids)

	servers := make([]RawServerData, 0, len(uuids))
	for _, uuid := range uuids {
		server := c.servers[uuid]
		processConfiguration, err := json.Marshal(server.Configuration.ProcessConfiguration)
		if err != nil {
			return nil, errors.Wrap(err, "remote/local: failed to marshal process configuration")
		}
		servers = append(servers, RawServerData{
			Uuid:                 server.UUID,
			Settings:             append([]byte(nil), server.Configuration.Settings...),
			ProcessConfiguration: processConfiguration,
		})
	}
	return servers, nil
}

func (c *localClient) ResetServersState(_ context.Context) error {
	return nil
}

func (c *localClient) GetServerConfiguration(ctx context.Context, uuid string) (ServerConfigurationResponse, error) {
	definition, err := c.GetLocalServer(ctx, uuid)
	if err != nil {
		return ServerConfigurationResponse{}, err
	}
	return definition.Configuration, nil
}

func (c *localClient) GetInstallationScript(ctx context.Context, uuid string) (InstallationScript, error) {
	definition, err := c.GetLocalServer(ctx, uuid)
	if err != nil {
		return InstallationScript{}, err
	}
	if definition.InstallationScript == nil {
		return InstallationScript{}, nil
	}
	return *cloneInstallationScript(definition.InstallationScript), nil
}

func (c *localClient) SetArchiveStatus(_ context.Context, _ string, _ bool) error {
	return nil
}

func (c *localClient) SetBackupStatus(_ context.Context, _ string, _ BackupRequest) error {
	return nil
}

func (c *localClient) SendRestorationStatus(_ context.Context, _ string, _ bool) error {
	return nil
}

func (c *localClient) SetInstallationStatus(_ context.Context, uuid string, data InstallStatusRequest) error {
	log.WithFields(log.Fields{
		"server":     uuid,
		"successful": data.Successful,
		"reinstall":  data.Reinstall,
	}).Debug("remote/local: recorded installation status")
	return nil
}

func (c *localClient) SetTransferStatus(_ context.Context, uuid string, successful bool) error {
	log.WithFields(log.Fields{
		"server":     uuid,
		"successful": successful,
	}).Debug("remote/local: recorded transfer status")
	return nil
}

func (c *localClient) ValidateSftpCredentials(_ context.Context, request SftpAuthRequest) (SftpAuthResponse, error) {
	var token localSftpToken
	verifyOptions := jwt.ValidatePayload(
		token.GetPayload(),
		jwt.ExpirationTimeValidator(time.Now()),
	)
	if _, err := jwt.Verify([]byte(request.Pass), config.GetJwtAlgorithm(), &token, verifyOptions); err != nil {
		return SftpAuthResponse{}, &SftpInvalidCredentialsError{}
	}

	if token.Server == "" || token.User == "" {
		return SftpAuthResponse{}, &SftpInvalidCredentialsError{}
	}

	if token.Username != "" && token.Username != request.User {
		return SftpAuthResponse{}, &SftpInvalidCredentialsError{}
	}

	c.mu.RLock()
	_, ok := c.servers[token.Server]
	c.mu.RUnlock()
	if !ok {
		return SftpAuthResponse{}, &SftpInvalidCredentialsError{}
	}

	return SftpAuthResponse{
		Server:      token.Server,
		User:        token.User,
		Permissions: append([]string(nil), token.Permissions...),
	}, nil
}

func (c *localClient) GetBackupRemoteUploadURLs(_ context.Context, _ string, _ int64) (BackupRemoteUploadResponse, error) {
	return BackupRemoteUploadResponse{}, errors.New("remote/local: remote upload URLs are not available")
}

func (c *localClient) SendActivityLogs(_ context.Context, _ []models.Activity) error {
	return nil
}

func (c *localClient) PushServerStateChange(_ context.Context, sid string, stateChange ServerStateChange) error {
	log.WithFields(log.Fields{
		"server": sid,
		"from":   stateChange.PrevState,
		"to":     stateChange.NewState,
	}).Debug("remote/local: recorded server state change")
	return nil
}

func cloneInstallationScript(script *InstallationScript) *InstallationScript {
	if script == nil {
		return nil
	}
	copy := *script
	return &copy
}

func cloneProcessConfiguration(configuration *ProcessConfiguration) *ProcessConfiguration {
	if configuration == nil {
		return nil
	}
	raw, err := json.Marshal(configuration)
	if err != nil {
		return configuration
	}
	var copy ProcessConfiguration
	if err := json.Unmarshal(raw, &copy); err != nil {
		return configuration
	}
	return &copy
}

func cloneLocalServerDefinition(definition LocalServerDefinition) LocalServerDefinition {
	return LocalServerDefinition{
		UUID: definition.UUID,
		Configuration: ServerConfigurationResponse{
			Settings:             append([]byte(nil), definition.Configuration.Settings...),
			ProcessConfiguration: cloneProcessConfiguration(definition.Configuration.ProcessConfiguration),
		},
		InstallationScript: cloneInstallationScript(definition.InstallationScript),
	}
}
