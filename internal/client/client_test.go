package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
)

func TestNew(t *testing.T) {
	c := New("https://api.example.com/", "tok_123", false)
	if c.BaseURL != "https://api.example.com" {
		t.Errorf("expected trailing slash trimmed, got %q", c.BaseURL)
	}
	if c.Token != "tok_123" {
		t.Errorf("expected token 'tok_123', got %q", c.Token)
	}
}

func TestGet_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer tok_test" {
			t.Error("expected Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "1", "name": "test"})
	}))
	defer server.Close()

	c := New(server.URL, "tok_test", false)
	resp, err := c.Get("/workspaces/1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Data == nil {
		t.Error("expected data to be parsed")
	}
}

func TestPost_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("expected Content-Type application/json")
		}
		w.Header().Set("Location", "/workspaces/2")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": "2"})
	}))
	defer server.Close()

	c := New(server.URL, "tok_test", false)
	resp, err := c.Post("/workspaces", map[string]string{"name": "new"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	if resp.Location != "/workspaces/2" {
		t.Errorf("expected location '/workspaces/2', got %q", resp.Location)
	}
}

func TestPost_RetriesRateLimitsWithFreshBody(t *testing.T) {
	t.Setenv("RECUERD0_RATE_WAIT", "1")
	var requestBodies [][]byte
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		requestBodies = append(requestBodies, body)
		if requests <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "slow down"})
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "2"})
	}))
	defer server.Close()

	var sleeps []time.Duration
	stderr := captureRateLimitStderr(t, func() {
		withRateLimitSleeper(t, func(duration time.Duration) {
			sleeps = append(sleeps, duration)
		})
		c := New(server.URL, "tok_test", false)
		response, err := c.Post("/workspaces", map[string]string{"name": "new"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
		}
	})

	wantBody, err := json.Marshal(map[string]string{"name": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
	for attempt, body := range requestBodies {
		if !bytes.Equal(body, wantBody) {
			t.Fatalf("request body %d = %q, want %q", attempt+1, body, wantBody)
		}
	}
	if want := []time.Duration{time.Second, time.Second}; !reflect.DeepEqual(sleeps, want) {
		t.Fatalf("sleeps = %v, want %v", sleeps, want)
	}
	wantStderr := "recuerd0: rate limited — waiting 1s before retrying (1/3)\n" +
		"recuerd0: rate limited — waiting 1s before retrying (2/3)\n"
	if stderr != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
	}
}

func TestPost_RateLimitRetriesExhausted(t *testing.T) {
	t.Setenv("RECUERD0_RATE_WAIT", "enabled")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "slow down"})
	}))
	defer server.Close()

	var sleeps []time.Duration
	var requestErr error
	stderr := captureRateLimitStderr(t, func() {
		withRateLimitSleeper(t, func(duration time.Duration) {
			sleeps = append(sleeps, duration)
		})
		_, requestErr = New(server.URL, "tok_test", false).Post("/workspaces", map[string]string{"name": "new"})
	})

	cliErr, ok := requestErr.(*clierrors.CLIError)
	if !ok {
		t.Fatalf("error = %T %v, want CLIError", requestErr, requestErr)
	}
	if cliErr.Code != clierrors.CodeRateLimited ||
		cliErr.Status != http.StatusTooManyRequests ||
		cliErr.ExitCode != clierrors.ExitRateLimited {
		t.Fatalf("rate-limit error changed: %#v", cliErr)
	}
	if requests != 4 {
		t.Fatalf("requests = %d, want 4", requests)
	}
	if want := []time.Duration{2 * time.Second, 2 * time.Second, 2 * time.Second}; !reflect.DeepEqual(sleeps, want) {
		t.Fatalf("sleeps = %v, want %v", sleeps, want)
	}
	wantStderr := "recuerd0: rate limited — waiting 2s before retrying (1/3)\n" +
		"recuerd0: rate limited — waiting 2s before retrying (2/3)\n" +
		"recuerd0: rate limited — waiting 2s before retrying (3/3)\n"
	if stderr != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
	}
}

func TestRetryAfterSeconds_DefaultsAndClamps(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		want       time.Duration
		wantSecond int
	}{
		{name: "missing", header: "", want: 60 * time.Second, wantSecond: 60},
		{name: "malformed", header: "tomorrow", want: 60 * time.Second, wantSecond: 60},
		{name: "zero", header: "0", want: time.Second, wantSecond: 1},
		{name: "negative", header: "-9", want: time.Second, wantSecond: 1},
		{name: "too large", header: "999", want: 120 * time.Second, wantSecond: 120},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RECUERD0_RATE_WAIT", "yes")
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if requests == 1 {
					if tt.header != "" {
						w.Header().Set("Retry-After", tt.header)
					}
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			var sleeps []time.Duration
			stderr := captureRateLimitStderr(t, func() {
				withRateLimitSleeper(t, func(duration time.Duration) {
					sleeps = append(sleeps, duration)
				})
				if _, err := New(server.URL, "tok_test", false).Get("/workspaces"); err != nil {
					t.Fatal(err)
				}
			})
			if requests != 2 {
				t.Fatalf("requests = %d, want 2", requests)
			}
			if want := []time.Duration{tt.want}; !reflect.DeepEqual(sleeps, want) {
				t.Fatalf("sleeps = %v, want %v", sleeps, want)
			}
			wantStderr := fmt.Sprintf(
				"recuerd0: rate limited — waiting %ds before retrying (1/3)\n",
				tt.wantSecond,
			)
			if stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
		})
	}
}

func TestRateLimitWaitCanBeDisabled(t *testing.T) {
	t.Setenv("RECUERD0_RATE_WAIT", "0")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "slow down"})
	}))
	defer server.Close()

	sleeps := 0
	var requestErr error
	stderr := captureRateLimitStderr(t, func() {
		withRateLimitSleeper(t, func(time.Duration) {
			sleeps++
		})
		_, requestErr = New(server.URL, "tok_test", false).Get("/workspaces")
	})

	cliErr, ok := requestErr.(*clierrors.CLIError)
	if !ok || cliErr.Code != clierrors.CodeRateLimited {
		t.Fatalf("error = %#v, want RATE_LIMITED", requestErr)
	}
	if requests != 1 || sleeps != 0 || stderr != "" {
		t.Fatalf("disabled wait retried or emitted output: requests=%d sleeps=%d stderr=%q", requests, sleeps, stderr)
	}
}

func TestDoRequest_DoesNotRetryOtherFailures(t *testing.T) {
	t.Setenv("RECUERD0_RATE_WAIT", "yes")

	t.Run("server error", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		sleeps := 0
		stderr := captureRateLimitStderr(t, func() {
			withRateLimitSleeper(t, func(time.Duration) {
				sleeps++
			})
			if _, err := New(server.URL, "tok_test", false).Get("/workspaces"); err == nil {
				t.Fatal("expected server error")
			}
		})
		if requests != 1 || sleeps != 0 || stderr != "" {
			t.Fatalf("server error retried: requests=%d sleeps=%d stderr=%q", requests, sleeps, stderr)
		}
	})

	t.Run("network error", func(t *testing.T) {
		transport := &countingErrorTransport{}
		c := New("https://api.example.com", "tok_test", false)
		c.HTTPClient = &http.Client{Transport: transport}

		sleeps := 0
		stderr := captureRateLimitStderr(t, func() {
			withRateLimitSleeper(t, func(time.Duration) {
				sleeps++
			})
			if _, err := c.Get("/workspaces"); err == nil {
				t.Fatal("expected network error")
			}
		})
		if transport.requests != 1 || sleeps != 0 || stderr != "" {
			t.Fatalf("network error retried: requests=%d sleeps=%d stderr=%q", transport.requests, sleeps, stderr)
		}
	})
}

func TestPatch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "1", "name": "updated"})
	}))
	defer server.Close()

	c := New(server.URL, "tok_test", false)
	resp, err := c.Patch("/workspaces/1", map[string]string{"name": "updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDelete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(204)
	}))
	defer server.Close()

	c := New(server.URL, "tok_test", false)
	resp, err := c.Delete("/memories/1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestGet_401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	}))
	defer server.Close()

	c := New(server.URL, "bad_token", false)
	_, err := c.Get("/workspaces")
	if err == nil {
		t.Fatal("expected error")
	}
	cliErr, ok := err.(*clierrors.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != clierrors.CodeAuth {
		t.Errorf("expected code %s, got %s", clierrors.CodeAuth, cliErr.Code)
	}
}

func TestGet_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer server.Close()

	c := New(server.URL, "tok_test", false)
	_, err := c.Get("/workspaces/999")
	if err == nil {
		t.Fatal("expected error")
	}
	cliErr, ok := err.(*clierrors.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != clierrors.CodeNotFound {
		t.Errorf("expected code %s, got %s", clierrors.CodeNotFound, cliErr.Code)
	}
}

type countingErrorTransport struct {
	requests int
}

func (transport *countingErrorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.requests++
	return nil, errors.New("network unavailable")
}

func withRateLimitSleeper(t *testing.T, sleeper func(time.Duration)) {
	t.Helper()
	previous := sleepAfterRateLimit
	sleepAfterRateLimit = sleeper
	t.Cleanup(func() {
		sleepAfterRateLimit = previous
	})
}

func captureRateLimitStderr(t *testing.T, run func()) string {
	t.Helper()
	previous := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	t.Cleanup(func() {
		os.Stderr = previous
		reader.Close()
		writer.Close()
	})

	run()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func TestGetWithPagination_LinkHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://api.example.com/workspaces?page=2>; rel="next"`)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{{"id": "1"}})
	}))
	defer server.Close()

	c := New(server.URL, "tok_test", false)
	resp, err := c.GetWithPagination("/workspaces")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.LinkNext != "https://api.example.com/workspaces?page=2" {
		t.Errorf("expected next URL, got %q", resp.LinkNext)
	}
}

func TestParseLinkNext(t *testing.T) {
	tests := []struct {
		header   string
		expected string
	}{
		{`<https://api.example.com/page?page=2>; rel="next"`, "https://api.example.com/page?page=2"},
		{`<https://api.example.com/page?page=1>; rel="prev", <https://api.example.com/page?page=3>; rel="next"`, "https://api.example.com/page?page=3"},
		{`<https://api.example.com/page?page=1>; rel="prev"`, ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := parseLinkNext(tt.header)
		if got != tt.expected {
			t.Errorf("parseLinkNext(%q) = %q, want %q", tt.header, got, tt.expected)
		}
	}
}

func TestExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		raw      []byte
		expected string
	}{
		{"error field", map[string]interface{}{"error": "bad request"}, nil, "bad request"},
		{"message field", map[string]interface{}{"message": "not found"}, nil, "not found"},
		{"errors array", map[string]interface{}{"errors": []interface{}{"invalid name"}}, nil, "invalid name"},
		{"raw body", nil, []byte("oops"), "oops"},
		{"empty", nil, nil, "unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractErrorMessage(tt.data, tt.raw)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
