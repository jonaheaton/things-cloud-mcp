package main

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
)

func TestIsConflictError(t *testing.T) {
	t.Run("typed 409", func(t *testing.T) {
		err := &thingscloud.HTTPStatusError{StatusCode: http.StatusConflict, Status: "409 Conflict"}
		if !isConflictError(err) {
			t.Fatal("expected typed 409 error to be treated as a conflict")
		}
	})

	t.Run("wrapped typed 409", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", &thingscloud.HTTPStatusError{StatusCode: http.StatusConflict, Status: "409 Conflict"})
		if !isConflictError(err) {
			t.Fatal("expected wrapped typed 409 error to be treated as a conflict")
		}
	})

	t.Run("non-409 status", func(t *testing.T) {
		err := &thingscloud.HTTPStatusError{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error"}
		if isConflictError(err) {
			t.Fatal("did not expect non-409 status to be treated as a conflict")
		}
	})

	t.Run("plain string mentioning 409", func(t *testing.T) {
		err := errors.New("Write failed: 409")
		if isConflictError(err) {
			t.Fatal("did not expect string matching alone to trigger conflict handling")
		}
	})
}
