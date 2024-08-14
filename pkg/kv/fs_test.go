package kv

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.uber.org/zap"
)

func TestSpec(t *testing.T) {
	Convey("Given a fresh context", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		test_path := t.TempDir()
		logger := zap.NewExample()
		store := NewFSStore(logger, test_path)
		ns := Namespace("namespace1")

		Convey("Values can be set", func() {
			key := "key1"
			value := "value1"
			err := store.Set(ctx, ns, key, value)
			So(err, ShouldEqual, nil)
		})
		Convey("When reading a set value", func() {
			key := "key2"
			value := "value1"
			store.Set(ctx, ns, key, value)
			returned, err := store.Get(ctx, ns, key)
			Convey("The application does not error", func() {
				So(err, ShouldEqual, nil)
			})
			Convey("The value read is equal to the value set", func() {
				So(returned, ShouldEqual, value)
			})
		})
		Convey("When reading a value that was not set", func() {
			key := "key3"
			returned, err := store.Get(ctx, ns, key)
			Convey("The application does not error", func() {
				So(err, ShouldEqual, nil)
			})
			Convey("The value is empty", func() {
				So(returned, ShouldEqual, "")
			})
		})
	})
}
