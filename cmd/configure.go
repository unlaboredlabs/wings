package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Minenetpro/pelican-wings/config"
	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var configureArgs struct {
	AllowedOrigin string
	TokenID       string
	Token         string
	ConfigPath    string
	Override      bool
	PanelURL      string
	Node          string
	AllowInsecure bool
}

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Generate a local Wings configuration",
	Run:   configureCmdRun,
}

func init() {
	configureCmd.PersistentFlags().StringVar(&configureArgs.AllowedOrigin, "allowed-origin", "", "Allowed browser origin for CORS and websocket access")
	configureCmd.PersistentFlags().StringVar(&configureArgs.TokenID, "token-id", "", "Identifier associated with the Wings API token (auto-generated if empty)")
	configureCmd.PersistentFlags().StringVarP(&configureArgs.Token, "token", "t", "", "Authentication token for Wings API requests (auto-generated if empty)")
	configureCmd.PersistentFlags().StringVarP(&configureArgs.ConfigPath, "config-path", "c", config.DefaultLocation, "The path where the configuration file should be made")
	configureCmd.PersistentFlags().BoolVar(&configureArgs.Override, "override", false, "Set to true to override an existing configuration for this node")
	configureCmd.PersistentFlags().StringVarP(&configureArgs.PanelURL, "panel-url", "p", "", "Deprecated alias for --allowed-origin")
	configureCmd.PersistentFlags().StringVarP(&configureArgs.Node, "node", "n", "", "Deprecated no-op flag")
	configureCmd.PersistentFlags().BoolVar(&configureArgs.AllowInsecure, "allow-insecure", false, "Deprecated no-op flag")
	_ = configureCmd.PersistentFlags().MarkDeprecated("panel-url", "use --allowed-origin if you need to allow a browser origin")
	_ = configureCmd.PersistentFlags().MarkDeprecated("node", "the local setup flow no longer uses node IDs")
	_ = configureCmd.PersistentFlags().MarkDeprecated("allow-insecure", "the local setup flow does not fetch remote configuration")
	_ = configureCmd.PersistentFlags().MarkHidden("panel-url")
	_ = configureCmd.PersistentFlags().MarkHidden("node")
	_ = configureCmd.PersistentFlags().MarkHidden("allow-insecure")
}

func configureCmdRun(cmd *cobra.Command, args []string) {
	if configureArgs.AllowedOrigin == "" {
		configureArgs.AllowedOrigin = strings.TrimSpace(configureArgs.PanelURL)
	}

	if _, err := os.Stat(configureArgs.ConfigPath); err == nil && !configureArgs.Override {
		err := huh.NewConfirm().
			Title("Override existing configuration file?").
			Value(&configureArgs.Override).
			Run()
		if err != nil {
			if err == huh.ErrUserAborted {
				return
			}
			panic(err)
		}
		if !configureArgs.Override {
			fmt.Println("Aborting process; a configuration file already exists for this node.")
			os.Exit(1)
		}
	} else if err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	if err := validateAllowedOrigin(configureArgs.AllowedOrigin); err != nil {
		panic(err)
	}

	if err := os.MkdirAll(filepath.Dir(configureArgs.ConfigPath), 0o755); err != nil {
		panic(err)
	}

	cfg, generatedTokenID, generatedToken, err := newLocalConfiguration(
		configureArgs.ConfigPath,
		configureArgs.TokenID,
		configureArgs.Token,
		configureArgs.AllowedOrigin,
	)
	if err != nil {
		panic(err)
	}

	if err = config.WriteToDisk(cfg); err != nil {
		panic(err)
	}

	fmt.Printf("Successfully wrote local Wings configuration to %s.\n", configureArgs.ConfigPath)
	fmt.Println("Wings API credentials:")
	fmt.Printf("  token_id: %s\n", cfg.AuthenticationTokenId)
	fmt.Printf("  token: %s\n", cfg.AuthenticationToken)
	if generatedTokenID || generatedToken {
		fmt.Println("Store these credentials securely. They are required for authenticated API access.")
	}
	if configureArgs.AllowedOrigin == "" {
		fmt.Println("No browser origin was configured. If a web UI is hosted on a different origin, set `allowed_origins` or rerun with --allowed-origin.")
	}
}

func newLocalConfiguration(configPath, tokenID, token, allowedOrigin string) (*config.Configuration, bool, bool, error) {
	cfg, err := config.NewAtPath(configPath)
	if err != nil {
		return nil, false, false, err
	}

	generatedTokenID := false
	if tokenID == "" {
		tokenID = uuid.NewString()
		generatedTokenID = true
	}

	generatedToken := false
	if token == "" {
		token, err = generateToken(32)
		if err != nil {
			return nil, false, false, err
		}
		generatedToken = true
	}

	cfg.Uuid = uuid.NewString()
	cfg.AuthenticationTokenId = tokenID
	cfg.AuthenticationToken = token
	if allowedOrigin != "" {
		cfg.AllowedOrigins = []string{allowedOrigin}
	}

	return cfg, generatedTokenID, generatedToken, nil
}

func generateToken(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func validateAllowedOrigin(origin string) error {
	if strings.TrimSpace(origin) == "" {
		return nil
	}

	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" {
		return fmt.Errorf("please provide a valid allowed origin")
	}

	return nil
}
