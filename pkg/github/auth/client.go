package auth

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"golang.org/x/oauth2"
)

func NewClient(config *Config) (*http.Client, error) {
	var client *http.Client
	switch config.Type {
	case TypeToken:
		client = oauth2.NewClient(
			context.Background(),
			oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.Token}),
		)

	case TypeApp:
		privKey := []byte(config.App.PrivateKey)
		if len(privKey) == 0 {
			key, err := ioutil.ReadFile(config.App.PrivateKeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load app key: %w", err)
			}
			privKey = key
		}

		rt, err := ghinstallation.New(http.DefaultTransport,
			config.App.AppID,
			config.App.InstallationID,
			privKey,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load app key: %w", err)
		}
		client = &http.Client{Transport: rt}

	default:
		return nil, fmt.Errorf("invalid auth type: %s", config.Type)
	}

	return client, nil
}
