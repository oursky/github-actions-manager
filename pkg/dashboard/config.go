package dashboard

import "github.com/oursky/github-actions-manager/pkg/utils/defaults"

type Config struct {
	Disabled  bool
	Addr      *string `validate:"omitempty,tcp_addr"`
	AssetsDir *string `validate:"omitempty,dir"`
}

func (c *Config) GetAddr() string {
	return defaults.Value(c.Addr, "127.0.0.1:8000")
}
