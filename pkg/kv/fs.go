package kv

import (
	"context"
	"os"
	"path"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type FSStore struct {
	fsPath string
}

func NewFSStore(logger *zap.Logger, fsPath string) *FSStore {
	return &FSStore{
		fsPath: fsPath,
	}
}

func (s *FSStore) Start(ctx context.Context, g *errgroup.Group) error {
	return nil
}

func (s *FSStore) Get(ctx context.Context, ns Namespace, key string) (string, error) {
	path := path.Join(s.fsPath, string(ns), key)

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			err := s.Touch(path)
			if err != nil {
				return "", err
			}
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func (s *FSStore) Set(ctx context.Context, ns Namespace, key string, value string) error {
	path := path.Join(s.fsPath, string(ns), key)

	err := s.Touch(path)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, []byte(value), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (s *FSStore) Touch(fpath string) error {
	baseDir := path.Dir(fpath)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(fpath, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}

	return nil
}
