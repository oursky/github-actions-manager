package kv

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestSetGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zap.NewExample()
	store := NewFSStore(logger, "testfs")
	ns := Namespace("namespace1")
	key := "key1"
	value := "value1"

	err := store.Set(ctx, ns, key, value)
	if err != nil {
		t.Fatalf("Error in set: %s", err)
	}

	returned, err := store.Get(ctx, ns, key)
	if err != nil {
		t.Fatalf("Error in get: %s", err)
	}
	if value != returned {
		t.Fatalf("Got something wrong: %s instead of %s", returned, value)
	}
}

func TestGetNone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zap.NewExample()
	store := NewFSStore(logger, "testfs")
	ns := Namespace("namespace1")
	key := "key2"

	returned, err := store.Get(ctx, ns, key)
	if err != nil {
		t.Fatalf("Store is not robust, err thrown by reading unstored value: %s", err)
	}
	if returned != "" {
		t.Fatalf("Got unstored value: %s", returned)
	}
}
