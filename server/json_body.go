package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxJSONBodyBytes = 1 << 20 // 1 MB

var errRequestBodyTooLarge = errors.New("request body too large")

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	reader := http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(reader)
	if err := dec.Decode(dst); err != nil {
		return normalizeJSONDecodeError(err)
	}
	return nil
}

func decodeOptionalJSONBody(w http.ResponseWriter, r *http.Request, dst any) (bool, error) {
	reader := http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(reader)
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, normalizeJSONDecodeError(err)
	}
	return true, nil
}

func normalizeJSONDecodeError(err error) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return errRequestBodyTooLarge
	}
	return err
}

func isRequestBodyTooLarge(err error) bool {
	return errors.Is(err, errRequestBodyTooLarge)
}

func writeJSONDecodeError(w http.ResponseWriter, prefix string, err error) {
	if isRequestBodyTooLarge(err) {
		jsonError(w, errRequestBodyTooLarge.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	jsonError(w, prefix+err.Error(), http.StatusBadRequest)
}
