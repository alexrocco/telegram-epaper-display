package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexrocco/telegram-epaper-display/backend/internal/render"
	"github.com/alexrocco/telegram-epaper-display/backend/internal/store"
)

func newTestFrame(t *testing.T) *Frame {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "store.json"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Add(store.Message{ID: 1, Text: "hello", Date: time.Now()}, 2); err != nil {
		t.Fatal(err)
	}
	return NewFrame(st, func() string { return "Test" }, "fallback", 5)
}

func TestFrameBinSizeAndETag(t *testing.T) {
	h := newTestFrame(t).Handler()

	req := httptest.NewRequest(http.MethodGet, "/frame.bin", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.Len(); got != render.BufferSize {
		t.Fatalf("body = %d bytes, want %d", got, render.BufferSize)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}

	// Second request with matching ETag must be 304 with no body.
	req2 := httptest.NewRequest(http.MethodGet, "/frame.bin", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Fatalf("304 body = %d bytes, want 0", rec2.Body.Len())
	}
}

func TestStatusOK(t *testing.T) {
	h := newTestFrame(t).Handler()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
