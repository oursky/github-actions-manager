package kv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type FSStore struct {
	lock   *sync.RWMutex
	fsPath string
}

func NewFSStore(logger *zap.Logger, fsPath string) *FSStore {
	return &FSStore{
		lock:   new(sync.RWMutex),
		fsPath: fsPath,
	}
}

func (s *FSStore) Start(ctx context.Context, g *errgroup.Group) error {
	return nil
}
func (s *FSStore) Get(ctx context.Context, ns Namespace, key string) (string, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	path := fmt.Sprintf("%s/%s/%s", s.fsPath, ns, key)
	b, err := os.ReadFile(path)

	if errors.Is(err, os.ErrNotExist) {
		s.Touch(path)
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *FSStore) Set(ctx context.Context, ns Namespace, key string, value string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	path := fmt.Sprintf("%s/%s/%s", s.fsPath, ns, key)
	s.Touch(path)
	return os.WriteFile(path, []byte(value), 0644)
}

func (s *FSStore) Touch(fpath string) error {
	baseDir := path.Dir(fpath)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}

	if file, err := os.OpenFile(fpath, os.O_RDONLY|os.O_CREATE, 0644); err != nil {
		return err
	} else {
		return file.Close()
	}
}
