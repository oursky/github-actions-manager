package slack

import "github.com/oursky/github-actions-manager/pkg/kv"

type Config struct {
	Disabled bool
	BotToken string `validate:"required_if=Disabled false"`
	AppToken string `validate:"required_if=Disabled false"`
}

var kvNamespace = kv.RegisterNamespace("slack-subscriptions")
