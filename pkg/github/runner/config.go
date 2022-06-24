package runner

import (
	"github.com/oursky/github-actions-manager/pkg/utils/tomltypes"
)

type Config struct {
	SyncInterval *tomltypes.Duration `toml:"syncInterval,omitempty"`
	SyncPageSize *int                `toml:"syncPageSize,omitempty" validate:"omitempty,min=1,max=100"`
}
