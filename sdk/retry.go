package clusdr

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func transient(err error) bool {
	if err == nil {
		return false
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.ResourceExhausted, codes.Aborted:
		return true
	default:
		return false
	}
}

func retry(ctx context.Context, fn func() error) error {
	backoff := 50 * time.Millisecond
	const maxBackoff = 2 * time.Second
	var last error
	for {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		last = fn()
		if last == nil || !transient(last) {
			return last
		}
		t := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			t.Stop()
			return last
		case <-t.C:
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
