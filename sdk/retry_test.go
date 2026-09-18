package clusdr

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTransient(t *testing.T) {
	if transient(nil) {
		t.Fatal("nil")
	}
	if !transient(status.Error(codes.Unavailable, "down")) {
		t.Fatal("Unavailable should retry")
	}
	if !transient(status.Error(codes.ResourceExhausted, "busy")) {
		t.Fatal("ResourceExhausted should retry")
	}
	if !transient(status.Error(codes.Aborted, "conflict")) {
		t.Fatal("Aborted should retry")
	}
	if transient(status.Error(codes.InvalidArgument, "bad")) {
		t.Fatal("InvalidArgument should not retry")
	}
}

func TestRetry_SucceedsAfterTransient(t *testing.T) {
	n := 0
	err := retry(context.Background(), func() error {
		n++
		if n < 3 {
			return status.Error(codes.Unavailable, "wait")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("calls: %d", n)
	}
}

func TestRetry_StopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := retry(ctx, func() error {
		return status.Error(codes.Unavailable, "down")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRetry_NonTransient(t *testing.T) {
	errBoom := status.Error(codes.InvalidArgument, "bad")
	err := retry(context.Background(), func() error { return errBoom })
	if err != errBoom {
		t.Fatalf("got %v", err)
	}
}

func TestRetry_StopsDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	err := retry(ctx, func() error {
		n++
		if n == 1 {
			go func() {
				time.Sleep(5 * time.Millisecond)
				cancel()
			}()
			return status.Error(codes.Unavailable, "down")
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected last transient error")
	}
}
