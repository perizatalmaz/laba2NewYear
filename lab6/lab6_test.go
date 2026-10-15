package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fixedNow is the "current time" used by the HTTP tests.
var fixedNow = time.Date(2026, time.October, 14, 15, 30, 0, 0, time.UTC)

// newTestServer starts a real HTTP server with the service handler
// and a fixed clock. The server is closed when the test finishes.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(newHandler(func() time.Time { return fixedNow }, logger))
	t.Cleanup(srv.Close)

	return srv
}

// get performs a GET request and fails the test if it cannot be made.
// The caller must close the response body.
func get(t *testing.T, url string) *http.Response {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}

	return resp
}

// checkJSONResponse checks the status code and content type of resp
// and decodes its body into v.
func checkJSONResponse(t *testing.T, resp *http.Response, wantStatus int, v any) {
	t.Helper()

	if resp.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", resp.StatusCode, wantStatus)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

func TestAPIDaysUntilNewYear(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name  string
		query string
		want  daysResponse
	}{
		{
			"no date uses current date",
			"",
			daysResponse{Date: "2026-10-14", NewYear: "2027-01-01", Days: 79},
		},
		{
			"explicit date",
			"?date=2025-12-31",
			daysResponse{Date: "2025-12-31", NewYear: "2026-01-01", Days: 1},
		},
		{
			"start of leap year",
			"?date=2024-01-01",
			daysResponse{Date: "2024-01-01", NewYear: "2025-01-01", Days: 366},
		},
		{
			"Feb 29",
			"?date=2024-02-29",
			daysResponse{Date: "2024-02-29", NewYear: "2025-01-01", Days: 307},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := get(t, srv.URL+"/api/v1/days-until-new-year"+tt.query)
			defer resp.Body.Close()

			var got daysResponse
			checkJSONResponse(t, resp, http.StatusOK, &got)

			if got != tt.want {
				t.Errorf("response = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAPIInvalidDate(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name  string
		query string
	}{
		{"empty date", "?date="},
		{"wrong format", "?date=14.10.2026"},
		{"not a date", "?date=tomorrow"},
		{"nonexistent day", "?date=2025-02-29"},
		{"nonexistent month", "?date=2025-13-01"},
		{"date with time", "?date=2025-10-14T10:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := get(t, srv.URL+"/api/v1/days-until-new-year"+tt.query)
			defer resp.Body.Close()

			var got errorResponse
			checkJSONResponse(t, resp, http.StatusBadRequest, &got)

			if got.Error == "" {
				t.Error("error message is empty")
			}
		})
	}
}

func TestAPIHealth(t *testing.T) {
	srv := newTestServer(t)

	resp := get(t, srv.URL+"/healthz")
	defer resp.Body.Close()

	var got healthResponse
	checkJSONResponse(t, resp, http.StatusOK, &got)

	if got.Status != "ok" {
		t.Errorf("status = %q, want %q", got.Status, "ok")
	}
}

func TestAPIMethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)

	resp, err := http.Post(srv.URL+"/api/v1/days-until-new-year", "text/plain", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestAPIUnknownPath(t *testing.T) {
	srv := newTestServer(t)

	resp := get(t, srv.URL+"/unknown")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
