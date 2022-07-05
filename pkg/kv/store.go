package kv

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Store interface {
	Start(ctx context.Context, g *errgroup.Group) error
	Get(ctx context.Context, ns Namespace, key string) (string, error)
	Set(ctx context.Context, ns Namespace, key string, value string) error
}

var namespaces map[string]struct{} = make(map[string]struct{})

type Namespace string

func RegisterNamespace(ns string) Namespace {
	namespaces[ns] = struct{}{}
	return Namespace(ns)
}
