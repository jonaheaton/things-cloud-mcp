package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/arthursoares/things-cloud-sdk/sync"
)

type stubInitialSyncer struct {
	changes []sync.Change
	err     error
}

func (s stubInitialSyncer) Sync() ([]sync.Change, error) {
	return s.changes, s.err
}

func TestRunInitialSync(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		if err := runInitialSync(stubInitialSyncer{changes: []sync.Change{}}); err != nil {
			t.Fatalf("runInitialSync returned error: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		err := runInitialSync(stubInitialSyncer{err: errors.New("boom")})
		if err == nil {
			t.Fatal("expected error")
		}
		if got, want := err.Error(), "initial sync failed: boom"; got != want {
			t.Fatalf("runInitialSync error = %q, want %q", got, want)
		}
	})
}

type stubShutdownServer struct {
	listenStarted  chan struct{}
	shutdownCalled chan struct{}
	listenErr      error
	shutdownErr    error
	waitForStop    bool
}

func (s *stubShutdownServer) ListenAndServe() error {
	if s.listenStarted != nil {
		close(s.listenStarted)
	}
	if s.waitForStop {
		<-s.shutdownCalled
	}
	return s.listenErr
}

func (s *stubShutdownServer) Shutdown(context.Context) error {
	if s.shutdownCalled != nil {
		select {
		case <-s.shutdownCalled:
		default:
			close(s.shutdownCalled)
		}
	}
	return s.shutdownErr
}

func TestServeWithGracefulShutdown(t *testing.T) {
	t.Parallel()

	t.Run("shuts down cleanly after context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		srv := &stubShutdownServer{
			listenStarted:  make(chan struct{}),
			shutdownCalled: make(chan struct{}),
			listenErr:      http.ErrServerClosed,
			waitForStop:    true,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- serveWithGracefulShutdown(ctx, srv, time.Second)
		}()

		<-srv.listenStarted
		cancel()

		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("serveWithGracefulShutdown returned error: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for shutdown")
		}

		select {
		case <-srv.shutdownCalled:
		default:
			t.Fatal("expected Shutdown to be called")
		}
	})

	t.Run("returns listen errors directly", func(t *testing.T) {
		t.Parallel()

		srv := &stubShutdownServer{
			shutdownCalled: make(chan struct{}),
			listenErr:      errors.New("listen failed"),
		}

		err := serveWithGracefulShutdown(context.Background(), srv, time.Second)
		if err == nil {
			t.Fatal("expected error")
		}
		if got, want := err.Error(), "listen failed"; got != want {
			t.Fatalf("serveWithGracefulShutdown error = %q, want %q", got, want)
		}

		select {
		case <-srv.shutdownCalled:
			t.Fatal("did not expect Shutdown to be called")
		default:
		}
	})
}
