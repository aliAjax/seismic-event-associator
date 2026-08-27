package domain

import (
	"context"
	"io"
)

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64) error
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}
