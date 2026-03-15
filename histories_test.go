package thingscloud

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testIdentifiable struct {
	id string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func (i testIdentifiable) UUID() string {
	return i.id
}

func TestClient_Histories(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{200, "histories-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		hs, err := c.Histories()
		if err != nil {
			t.Fatalf("Expected history request to succeed, but didn't: %q", err.Error())
		}
		if len(hs) != 1 {
			t.Errorf("Expected to receive %d histories, but got %d", 1, len(hs))
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		server := fakeBodyServer(200, "{")
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		if _, err := c.Histories(); err == nil {
			t.Fatal("Expected malformed histories JSON to fail")
		}
	})

	t.Run("Error", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{401, "error.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "unknown@example.com", "")
		_, err := c.Histories()
		if err == nil {
			t.Error("Expected history request to fail, but didn't")
		}
	})
}

func TestClient_CreateHistory(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{200, "create-history-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h, err := c.CreateHistory()
		if err != nil {
			t.Fatalf("Expected request to succeed, but didn't: %q", err.Error())
		}
		if h.ID != "33333abb-bfe4-4b03-a5c9-106d42220c72" {
			t.Fatalf("Expected key %s but got %s", "33333abb-bfe4-4b03-a5c9-106d42220c72", h.ID)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		server := fakeBodyServer(200, "{")
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		if _, err := c.CreateHistory(); err == nil {
			t.Fatal("Expected malformed create history JSON to fail")
		}
	})
}

func TestHistory_Delete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{202, "create-history-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h := History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
		err := h.Delete()
		if err != nil {
			t.Fatalf("Expected request to succeed, but didn't: %q", err.Error())
		}
	})
}

func TestHistory_Sync(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		server := fakeServer(fakeResponse{200, "history-success.json"})
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h := History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
		err := h.Sync()
		if err != nil {
			t.Fatalf("Expected request to succeed, but didn't: %q", err.Error())
		}
		if h.LatestServerIndex != 27 {
			t.Errorf("Expected LatestServerIndex of %d, but got %d", 27, h.LatestServerIndex)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		server := fakeBodyServer(200, "{")
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h := History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
		if err := h.Sync(); err == nil {
			t.Fatal("Expected malformed sync JSON to fail")
		}
	})
}

func TestHistory_Write(t *testing.T) {
	t.Run("StatusError", func(t *testing.T) {
		t.Parallel()

		c := New("https://example.com", "martin@example.com", "")
		c.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusConflict,
				Status:     "409 Conflict",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"conflict"}`)),
				Request:    req,
			}, nil
		})

		h := History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72", LatestServerIndex: 1}
		err := h.Write(testIdentifiable{id: "abc123"})
		if err == nil {
			t.Fatal("expected conflict write to fail")
		}

		var statusErr *HTTPStatusError
		if !errors.As(err, &statusErr) {
			t.Fatalf("expected HTTPStatusError, got %T: %v", err, err)
		}
		if statusErr.StatusCode != http.StatusConflict {
			t.Fatalf("expected status code %d, got %d", http.StatusConflict, statusErr.StatusCode)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		t.Parallel()
		server := fakeBodyServer(200, "{")
		defer server.Close()

		c := New(fmt.Sprintf("http://%s", server.Listener.Addr().String()), "martin@example.com", "")
		h := History{Client: c, ID: "33333abb-bfe4-4b03-a5c9-106d42220c72"}
		if err := h.Write(testIdentifiable{id: "abc123"}); err == nil {
			t.Fatal("Expected malformed commit JSON to fail")
		}
	})

	t.Run("InvalidRequest", func(t *testing.T) {
		t.Parallel()
		c := New("http://example.com", "martin@example.com", "")
		h := History{Client: c, ID: "bad\nid"}
		if err := h.Write(testIdentifiable{id: "abc123"}); err == nil {
			t.Fatal("Expected invalid history ID to return an error")
		}
	})
}
