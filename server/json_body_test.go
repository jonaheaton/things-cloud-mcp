package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleCreateTask_RequestBodyTooLarge(t *testing.T) {
	oversizedTitle := strings.Repeat("a", maxJSONBodyBytes)
	body := `{"title":"` + oversizedTitle + `"}`

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/create", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handleCreateTask(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), errRequestBodyTooLarge.Error()) {
		t.Fatalf("expected response to mention %q, got %q", errRequestBodyTooLarge.Error(), rec.Body.String())
	}
}

func TestDecodeOptionalJSONBody_RequestBodyTooLarge(t *testing.T) {
	oversizedKey := strings.Repeat("a", maxJSONBodyBytes)
	body := `{"key":"` + oversizedKey + `"}`

	req := httptest.NewRequest(http.MethodPost, "/api/debug/delete-history", strings.NewReader(body))
	rec := httptest.NewRecorder()

	var payload struct {
		Key string `json:"key"`
	}
	ok, err := decodeOptionalJSONBody(rec, req, &payload)
	if ok {
		t.Fatal("expected oversized optional body not to decode successfully")
	}
	if !isRequestBodyTooLarge(err) {
		t.Fatalf("expected request body too large error, got %v", err)
	}
}
