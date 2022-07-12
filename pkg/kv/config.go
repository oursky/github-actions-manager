package kv

import (
	"fmt"

	"go.uber.org/zap"
)

type Type string

const (
	TypeInMemory      Type = "InMemory"
	TypeKubeConfigMap Type = "KubeConfigMap"
)

type Config struct {
	Type          Type   `validate:"required,oneof=InMemory KubeConfigMap"`
	KubeNamespace string `validate:"required_if=Type KubeConfigMap"`
}

func NewStore(logger *zap.Logger, config *Config) (Store, error) {
	switch config.Type {
	case TypeInMemory:
		return NewInMemoryStore(), nil

	case TypeKubeConfigMap:
		return NewKubeConfigMapStore(logger, config.KubeNamespace)
	}
	return nil, fmt.Errorf("invalid kv store type: %s", config.Type)
}
