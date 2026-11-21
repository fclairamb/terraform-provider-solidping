package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRequestAndEnvelope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pat_x" {
			t.Errorf("Authorization = %q", got)
		}

		if r.URL.Path != "/api/v1/orgs/acme/channels" {
			t.Errorf("path = %q", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"uid": "u1", "name": "hook"}}})
	}))
	defer srv.Close()

	channels, err := New(srv.URL+"/", "acme", "pat_x", "test").ListChannels(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(channels) != 1 || channels[0].UID != "u1" || channels[0].Name != "hook" {
		t.Errorf("channels = %+v", channels)
	}
}

func TestClientErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		body     string
		notFound bool
		contains []string
	}{
		{
			name:     "not found",
			status:   http.StatusNotFound,
			body:     `{"title":"Check not found","code":"CHECK_NOT_FOUND","detail":"check not found"}`,
			notFound: true,
			contains: []string{"Check not found", "CHECK_NOT_FOUND", "HTTP 404"},
		},
		{
			name:     "validation with fields",
			status:   http.StatusUnprocessableEntity,
			body:     `{"title":"Configuration validation failed","code":"VALIDATION_ERROR","fields":[{"name":"url","message":"is required"}]}`,
			contains: []string{"Configuration validation failed", "url: is required", "VALIDATION_ERROR"},
		},
		{
			name:     "not JSON",
			status:   http.StatusBadGateway,
			body:     "upstream unavailable",
			contains: []string{"Bad Gateway", "upstream unavailable", "HTTP 502"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := New(srv.URL, "acme", "pat_x", "test").GetCheck(context.Background(), "u1")
			if err == nil {
				t.Fatal("expected an error")
			}

			if IsNotFound(err) != tt.notFound {
				t.Errorf("IsNotFound = %v, want %v", IsNotFound(err), tt.notFound)
			}

			for _, want := range tt.contains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}
