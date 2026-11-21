package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRefreshPublic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		prior       map[string]any
		server      map[string]any
		privateKeys []string
		want        map[string]any
	}{
		{
			name:        "import takes the server value, minus the secrets",
			server:      map[string]any{"url": "https://acme.com", "method": "GET", "signingSecret": "generated"},
			privateKeys: []string{"signingSecret"},
			want:        map[string]any{"url": "https://acme.com", "method": "GET"},
		},
		{
			name:   "server-only keys are ignored",
			prior:  map[string]any{"url": "https://acme.com"},
			server: map[string]any{"url": "https://acme.com", "signingSecret": "generated"},
			want:   map[string]any{"url": "https://acme.com"},
		},
		{
			name:   "drift on a managed key is reported",
			prior:  map[string]any{"url": "https://acme.com"},
			server: map[string]any{"url": "https://acme.com/changed"},
			want:   map[string]any{"url": "https://acme.com/changed"},
		},
		{
			name:        "a secret written in the public half is kept",
			prior:       map[string]any{"url": "https://acme.com", "token": "s3cret"},
			server:      map[string]any{"url": "https://acme.com"},
			privateKeys: []string{"token"},
			want:        map[string]any{"url": "https://acme.com", "token": "s3cret"},
		},
		{
			name:   "a key removed out of band is dropped",
			prior:  map[string]any{"url": "https://acme.com", "method": "POST"},
			server: map[string]any{"url": "https://acme.com"},
			want:   map[string]any{"url": "https://acme.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := refreshPublic(tt.prior, tt.server, tt.privateKeys); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeSecrets(t *testing.T) {
	t.Parallel()

	got := mergeSecrets(
		map[string]any{"url": "https://acme.com", "token": "public"},
		map[string]any{"token": "s3cret"},
		map[string]any{"token": "old", "password": "old"},
	)

	want := map[string]any{"url": "https://acme.com", "token": "s3cret", "password": nil}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOptionalString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		server string
		prior  types.String
		want   types.String
	}{
		{"value", "hello", types.StringNull(), types.StringValue("hello")},
		{"empty stays null", "", types.StringNull(), types.StringNull()},
		{"empty stays empty", "", types.StringValue(""), types.StringValue("")},
		{"cleared out of band", "", types.StringValue("hello"), types.StringNull()},
		{"unknown becomes null", "", types.StringUnknown(), types.StringNull()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := optionalString(tt.server, tt.prior); !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
