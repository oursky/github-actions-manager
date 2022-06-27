package auth

import (
	"fmt"
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"golang.org/x/oauth2"
)

type TokenTransport struct {
	*oauth2.Transport
}

type AppTransport struct {
	*ghinstallation.Transport
	AppsTransport *ghinstallation.AppsTransport
}

func NewTransport(config *Config, base http.RoundTripper) (http.RoundTripper, error) {
	var transport http.RoundTripper
	switch config.Type {
	case TypeToken:
		transport = TokenTransport{
			Transport: &oauth2.Transport{
				Base:   base,
				Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.Token}),
			},
		}

	case TypeApp:
		privKey := []byte(config.App.PrivateKey)
		if len(privKey) == 0 {
			key, err := os.ReadFile(config.App.PrivateKeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load app key: %w", err)
			}
			privKey = key
		}

		appTransport, err := ghinstallation.NewAppsTransport(base,
			config.App.AppID,
			privKey,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load app key: %w", err)
		}

		transport = AppTransport{
			Transport: ghinstallation.NewFromAppsTransport(
				appTransport,
				config.App.InstallationID,
			),
			AppsTransport: appTransport,
		}

	default:
		return nil, fmt.Errorf("invalid auth type: %s", config.Type)
	}

	return transport, nil
}
