package dashboard

type Config struct {
	Addr      *string `toml:"addr,omitempty" validate:"omitempty,tcp_addr"`
	AssetsDir *string `toml:"assetsDir,omitempty" validate:"omitempty,dir"`
}
