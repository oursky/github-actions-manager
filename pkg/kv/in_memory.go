package kv

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

type InMemoryStore struct {
	lock   *sync.RWMutex
	values map[string]map[string]string
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{lock: new(sync.RWMutex)}
}

func (s *InMemoryStore) Start(ctx context.Context, g *errgroup.Group) error {
	return nil
}

func (s *InMemoryStore) Get(ctx context.Context, ns Namespace, key string) (string, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.values[string(ns)][key], nil
}

func (s *InMemoryStore) Set(ctx context.Context, ns Namespace, key string, value string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.values == nil {
		s.values = make(map[string]map[string]string)
	}

	nsMap := s.values[string(ns)]
	if nsMap == nil {
		nsMap = make(map[string]string)
		s.values[string(ns)] = nsMap
	}

	nsMap[key] = value
	return nil
}
