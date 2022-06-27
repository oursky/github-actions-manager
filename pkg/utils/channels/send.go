package channels

import "context"

func Send[T any](ctx context.Context, c chan<- T, value T) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c <- value:
		return nil
	}
}
