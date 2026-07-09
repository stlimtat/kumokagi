package azure_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/stlimtat/kumokagi/internal/azure"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// fakeCredential returns a dummy token so the SDK auth policy doesn't fail.
type fakeCredential struct{}

func (f *fakeCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fake-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

// newMockTLSServer starts a mock Azure Key Vault TLS server on 127.0.0.1 (IPv4 only).
// Azure SDK enforces HTTPS, so we must use TLS even for local mocks.
func newMockTLSServer(t *testing.T, mux http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = ln
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func buildDefaultMux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/secrets/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/secrets/"), "/")
		name := parts[0]

		switch r.Method {
		case http.MethodGet:
			if name == "prod--myapp--missing" {
				writeAzureError(w, http.StatusNotFound, "SecretNotFound", "secret not found")
				return
			}
			if name == "prod--myapp--db_password" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(secretResponse(name, "s3cr3t"))
				return
			}
			writeAzureError(w, http.StatusNotFound, "SecretNotFound", "secret not found")

		case http.MethodPut:
			var body struct {
				Value string `json:"value"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(secretResponse(name, body.Value))

		case http.MethodDelete:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   fmt.Sprintf("https://mock.vault.azure.net/secrets/%s/abc123", name),
				"name": name,
			})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// DELETE /deletedsecrets/{name} — purge
	mux.HandleFunc("/deletedsecrets/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	return mux
}

// secretResponse builds a minimal Azure Key Vault secret JSON response.
func secretResponse(name, value string) map[string]any {
	return map[string]any{
		"value": value,
		"id":    fmt.Sprintf("https://mock.vault.azure.net/secrets/%s/abc123", name),
	}
}

// writeAzureError writes an Azure-style error response.
func writeAzureError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": code, "message": msg},
	})
}

func newTestClient(t *testing.T) *azure.Client {
	t.Helper()
	srv := newMockTLSServer(t, buildDefaultMux())
	opts := &azsecrets.ClientOptions{}
	opts.Transport = srv.Client()
	c, err := azure.NewWithCredential(srv.URL, &fakeCredential{}, opts)
	require.NoError(t, err)
	return c
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx := t.Context()

	t.Run("returns value for existing secret", func(t *testing.T) {
		t.Parallel()
		val, err := client.Get(ctx, provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", val)
	})

	t.Run("returns ErrNotFound for missing secret", func(t *testing.T) {
		t.Parallel()
		_, err := client.Get(ctx, provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
		require.ErrorIs(t, err, provider.ErrNotFound)
	})
}

func TestClient_Set(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	err := client.Set(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "newkey"}, "newvalue")
	require.NoError(t, err)
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	err := client.Delete(t.Context(), provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
	require.NoError(t, err)
}

func TestClient_List(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/secrets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{"id": "https://mock.vault.azure.net/secrets/prod--myapp--db_password"},
				{"id": "https://mock.vault.azure.net/secrets/prod--myapp--api_key"},
				{"id": "https://mock.vault.azure.net/secrets/prod--otherapp--token"}, // different app — filtered out
			},
		})
	})

	srv := newMockTLSServer(t, mux)
	opts := &azsecrets.ClientOptions{}
	opts.Transport = srv.Client()
	client, err := azure.NewWithCredential(srv.URL, &fakeCredential{}, opts)
	require.NoError(t, err)

	keys, err := client.List(t.Context(), provider.SecretPath{Env: "prod", App: "myapp"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"db_password", "api_key"}, keys)
}

func TestClient_Exists(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx := t.Context()

	t.Run("true for existing secret", func(t *testing.T) {
		t.Parallel()
		ok, err := client.Exists(ctx, provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"})
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("false for missing secret", func(t *testing.T) {
		t.Parallel()
		ok, err := client.Exists(ctx, provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"})
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
