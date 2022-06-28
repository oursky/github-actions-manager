package kv

import "context"

type Store interface {
	Get(ctx context.Context, ns Namespace, key string) (string, error)
	Set(ctx context.Context, ns Namespace, key string, value string) error
}

var namespaces map[string]struct{} = make(map[string]struct{})

type Namespace string

func RegisterNamespace(ns string) Namespace {
	namespaces[ns] = struct{}{}
	return Namespace(ns)
}
