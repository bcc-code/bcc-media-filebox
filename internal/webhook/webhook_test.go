package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendPostsJSON(t *testing.T) {
	var gotCT string
	var got Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := Send(context.Background(), srv.URL, Payload{
		Sidecar: "ARR_SUB_NAME.mov.json",
		Path:    "RawMaterial/ARR_SUB_NAME.mov.json",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotCT)
	}
	if got.Sidecar != "ARR_SUB_NAME.mov.json" {
		t.Errorf("sidecar = %q", got.Sidecar)
	}
	if got.Path != "RawMaterial/ARR_SUB_NAME.mov.json" {
		t.Errorf("path = %q", got.Path)
	}
}

func TestSendErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := Send(context.Background(), srv.URL, Payload{Sidecar: "x.json", Path: "x.json"}); err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}
