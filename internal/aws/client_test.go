package aws_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
	internaws "github.com/stlimtat/kumokagi/internal/aws"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// staticEndpoint routes all SDK calls to the mock server.
type staticEndpoint struct{ url string }

func (s staticEndpoint) ResolveEndpoint(ctx context.Context, params secretsmanager.EndpointParameters) (smithyendpoints.Endpoint, error) {
	u, _ := url.Parse(s.url)
	return smithyendpoints.Endpoint{URI: *u}, nil
}

func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	store := map[string]string{
		"prod/myapp/db_password": `{"value":"s3cr3t"}`,
	}
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		mu.Lock()
		defer mu.Unlock()

		switch target {
		case "secretsmanager.GetSecretValue":
			var req struct {
				SecretId string
			}
			json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
			val, ok := store[req.SecretId]
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"__type": "ResourceNotFoundException",
					"Message": "secret not found",
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"SecretString": val}) //nolint:errcheck

		case "secretsmanager.CreateSecret":
			var req struct {
				Name         string
				SecretString string
			}
			json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
			if _, exists := store[req.Name]; exists {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"__type":  "ResourceExistsException",
					"Message": "already exists",
				})
				return
			}
			store[req.Name] = req.SecretString
			json.NewEncoder(w).Encode(map[string]string{"ARN": "arn:aws:secretsmanager:us-east-1:000000000000:secret:" + req.Name}) //nolint:errcheck

		case "secretsmanager.PutSecretValue":
			var req struct {
				SecretId     string
				SecretString string
			}
			json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
			store[req.SecretId] = req.SecretString
			json.NewEncoder(w).Encode(map[string]string{"ARN": "arn:aws:secretsmanager:us-east-1:000000000000:secret:" + req.SecretId}) //nolint:errcheck

		case "secretsmanager.DeleteSecret":
			var req struct {
				SecretId string
			}
			json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
			delete(store, req.SecretId)
			json.NewEncoder(w).Encode(map[string]string{"ARN": "arn:aws:secretsmanager:us-east-1:000000000000:secret:" + req.SecretId}) //nolint:errcheck

		case "secretsmanager.DescribeSecret":
			var req struct {
				SecretId string
			}
			json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
			if _, ok := store[req.SecretId]; !ok {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"__type":  "ResourceNotFoundException",
					"Message": "secret not found",
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"Name": req.SecretId}) //nolint:errcheck

		case "secretsmanager.ListSecrets":
			type secretEntry struct {
				Name string
			}
			var list []secretEntry
			for k := range store {
				list = append(list, secretEntry{Name: k})
			}
			json.NewEncoder(w).Encode(map[string]any{"SecretList": list}) //nolint:errcheck

		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	// ponytail: IPv4 explicit — sandbox blocks IPv6
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(t *testing.T, srv *httptest.Server) *internaws.Client {
	t.Helper()
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(awssdk.AnonymousCredentials{}),
	)
	require.NoError(t, err)
	svc := secretsmanager.NewFromConfig(cfg,
		secretsmanager.WithEndpointResolverV2(staticEndpoint{url: srv.URL}),
	)
	return internaws.NewWithSvc(svc)
}

var testPath = provider.SecretPath{Env: "prod", App: "myapp", Key: "db_password"}
var missingPath = provider.SecretPath{Env: "prod", App: "myapp", Key: "missing"}

func TestGet(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t)
	client := newTestClient(t, srv)
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		val, err := client.Get(ctx, testPath)
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", val)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		_, err := client.Get(ctx, missingPath)
		require.ErrorIs(t, err, provider.ErrNotFound)
	})
}

func TestSet(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t)
	client := newTestClient(t, srv)
	ctx := context.Background()

	t.Run("create new secret", func(t *testing.T) {
		t.Parallel()
		err := client.Set(ctx, provider.SecretPath{Env: "prod", App: "myapp", Key: "newkey"}, "newvalue")
		require.NoError(t, err)
	})

	t.Run("update existing secret", func(t *testing.T) {
		t.Parallel()
		err := client.Set(ctx, testPath, "updated")
		require.NoError(t, err)
	})
}

func TestDelete(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t)
	client := newTestClient(t, srv)
	err := client.Delete(context.Background(), testPath)
	require.NoError(t, err)
}

func TestList(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t)
	client := newTestClient(t, srv)
	keys, err := client.List(context.Background(), provider.SecretPath{Env: "prod", App: "myapp"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"db_password"}, keys)
}

func TestExists(t *testing.T) {
	t.Parallel()
	srv := newMockServer(t)
	client := newTestClient(t, srv)
	ctx := context.Background()

	t.Run("true", func(t *testing.T) {
		t.Parallel()
		ok, err := client.Exists(ctx, testPath)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("false", func(t *testing.T) {
		t.Parallel()
		ok, err := client.Exists(ctx, missingPath)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
