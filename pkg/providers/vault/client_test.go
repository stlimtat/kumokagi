package vault_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stlimtat/kumokagi/pkg/providers/vault"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func newMockVaultServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/secret/data/prod/myapp/db_password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"data": map[string]any{"value": "s3cr3t"},
			},
		})
	})

	mux.HandleFunc("/v1/secret/data/prod/myapp/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mux.HandleFunc("/v1/secret/data/prod/myapp/newkey", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	})

	mux.HandleFunc("/v1/secret/metadata/prod/myapp/db_password", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"versions": map[string]any{}}})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/secret/metadata/prod/myapp/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mux.HandleFunc("/v1/secret/metadata/prod/myapp", func(w http.ResponseWriter, r *http.Request) {
		// The Vault SDK sends either LIST or GET?list=true depending on version.
		isList := r.Method == "LIST" || (r.Method == http.MethodGet && r.URL.Query().Get("list") == "true")
		if !isList {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"keys": []string{"db_password", "api_key"},
			},
		})
	})

	// Use explicit IPv4 listener — sandbox blocks IPv6 binding.
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T) *vault.Client {
	t.Helper()
	srv := newMockVaultServer(t)
	client, err := vault.NewWithAddress(t.Context(), srv.URL)
	require.NoError(t, err)
	return client
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx := t.Context()

	t.Run("returns value for existing secret", func(t *testing.T) {
		t.Parallel()
		val, err := client.Get(ctx, provider.SecretPath{
			Mount: "secret", Env: "prod", App: "myapp", Key: "db_password",
		})
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", val)
	})

	t.Run("returns ErrNotFound for missing secret", func(t *testing.T) {
		t.Parallel()
		_, err := client.Get(ctx, provider.SecretPath{
			Mount: "secret", Env: "prod", App: "myapp", Key: "missing",
		})
		require.ErrorIs(t, err, provider.ErrNotFound)
	})
}

func TestClient_Set(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	err := client.Set(t.Context(), provider.SecretPath{
		Mount: "secret", Env: "prod", App: "myapp", Key: "newkey",
	}, "newvalue")
	require.NoError(t, err)
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	err := client.Delete(t.Context(), provider.SecretPath{
		Mount: "secret", Env: "prod", App: "myapp", Key: "db_password",
	})
	require.NoError(t, err)
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	keys, err := client.List(t.Context(), provider.SecretPath{
		Mount: "secret", Env: "prod", App: "myapp",
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"db_password", "api_key"}, keys)
}

func TestClient_Exists(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx := t.Context()

	t.Run("true for existing secret", func(t *testing.T) {
		t.Parallel()
		ok, err := client.Exists(ctx, provider.SecretPath{
			Mount: "secret", Env: "prod", App: "myapp", Key: "db_password",
		})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("false for missing secret", func(t *testing.T) {
		t.Parallel()
		ok, err := client.Exists(ctx, provider.SecretPath{
			Mount: "secret", Env: "prod", App: "myapp", Key: "missing",
		})
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
