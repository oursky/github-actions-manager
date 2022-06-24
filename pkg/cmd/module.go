package cmd

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Module interface {
	Start(context.Context, *errgroup.Group) error
}
