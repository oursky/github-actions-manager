package auth

type Type string

const (
	TypeToken Type = "Token"
	TypeApp   Type = "App"
)

type Config struct {
	Type  Type           `validate:"required,oneof=Token App"`
	Token string         `validate:"required_if=Type Token"`
	App   *AppAuthConfig `validate:"required_if=Type App"`
}

type AppAuthConfig struct {
	AppID          int64 `validate:"required"`
	InstallationID int64 `validate:"required"`
	PrivateKey     string
	PrivateKeyPath string `validate:"required_without=PrivateKey,omitempty,file"`
}
