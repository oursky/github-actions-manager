package kv

import (
	"fmt"

	"go.uber.org/zap"
)

type Type string

const (
	TypeFS            Type = "FS"
	TypeInMemory      Type = "InMemory"
	TypeKubeConfigMap Type = "KubeConfigMap"
)

type Config struct {
	Type          Type   `validate:"required,oneof=InMemory KubeConfigMap FS"`
	KubeNamespace string `validate:"required_if=Type KubeConfigMap"`
	FSPath        string `validate:"required_if=Type FS"`
}

func NewStore(logger *zap.Logger, config *Config) (Store, error) {
	switch config.Type {
	case TypeFS:
		return NewFSStore(logger, config.FSPath), nil

	case TypeInMemory:
		return NewInMemoryStore(), nil

	case TypeKubeConfigMap:
		return NewKubeConfigMapStore(logger, config.KubeNamespace)
	}
	return nil, fmt.Errorf("invalid kv store type: %s", config.Type)
}
