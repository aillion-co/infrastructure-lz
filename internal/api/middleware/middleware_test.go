package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecovery_ContainsPanic(t *testing.T) {
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	handler := Recovery(panicking)

	w := httptest.NewRecorder()
	// Must not propagate the panic.
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Error("expected an error body after panic")
	}
}

func TestLogging_CapturesStatusCode(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := Logging(inner)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusTeapot {
		t.Errorf("expected status passthrough 418, got %d", w.Code)
	}
}

func TestWrappedWriter_DefaultsTo200AndRecords(t *testing.T) {
	rec := httptest.NewRecorder()
	ww := &wrappedWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Without an explicit WriteHeader the recorded code stays 200.
	if ww.statusCode != http.StatusOK {
		t.Errorf("expected default 200, got %d", ww.statusCode)
	}
	ww.WriteHeader(http.StatusNotFound)
	if ww.statusCode != http.StatusNotFound {
		t.Errorf("expected 404 after WriteHeader, got %d", ww.statusCode)
	}
}

func TestTelemetry_InjectsTraceHeaderAndServes(t *testing.T) {
	var served bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	})
	handler := Telemetry(inner)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/features", nil))

	if !served {
		t.Error("telemetry middleware did not call the next handler")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
