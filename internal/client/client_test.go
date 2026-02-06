package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/errors"
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
	cliErr, ok := err.(*errors.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != errors.CodeAuth {
		t.Errorf("expected code %s, got %s", errors.CodeAuth, cliErr.Code)
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
	cliErr, ok := err.(*errors.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if cliErr.Code != errors.CodeNotFound {
		t.Errorf("expected code %s, got %s", errors.CodeNotFound, cliErr.Code)
	}
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
