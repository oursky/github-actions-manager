package auth

type Type string

const (
	TypeToken Type = "Token"
	TypeApp   Type = "App"
)

type Config struct {
	Type  Type           `toml:"type" validate:"required,oneof=Token App"`
	Token string         `toml:"token,omitempty" validate:"required_if=Type Token"`
	App   *AppAuthConfig `toml:"app,omitempty" validate:"required_if=Type App"`
}

type AppAuthConfig struct {
	AppID          int64  `toml:"appID" validate:"required"`
	InstallationID int64  `toml:"installationID" validate:"required"`
	PrivateKey     string `toml:"privateKey,omitempty"`
	PrivateKeyPath string `toml:"privateKeyPath,omitempty" validate:"required_without=PrivateKey,omitempty,file"`
}
