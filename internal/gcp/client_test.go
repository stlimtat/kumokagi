package gcp_test

import (
	"context"
	"net"
	"testing"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/stlimtat/kumokagi/internal/gcp"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

const (
	testProject = "test-project"
	testEnv     = "prod"
	testApp     = "myapp"
	testKey     = "db_password"
	testValue   = "s3cr3t"
)

// testPath is the standard SecretPath used across tests.
var testPath = provider.SecretPath{Env: testEnv, App: testApp, Key: testKey}

// mockServer implements secretmanagerpb.SecretManagerServiceServer for testing.
type mockServer struct {
	secretmanagerpb.UnimplementedSecretManagerServiceServer

	secrets  map[string]string   // name -> latest value
	versions map[string][]string // name -> ordered values
}

func newMockServer() *mockServer {
	m := &mockServer{
		secrets:  make(map[string]string),
		versions: make(map[string][]string),
	}
	// Pre-populate an existing secret.
	m.secrets["prod--myapp--db_password"] = testValue
	m.versions["prod--myapp--db_password"] = []string{testValue}
	return m
}

func (m *mockServer) secretResourceName(name string) string {
	return "projects/" + testProject + "/secrets/" + name
}

func (m *mockServer) GetSecret(_ context.Context, req *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
	// Extract the short name from the full resource path.
	name := shortName(req.Name)
	if _, ok := m.secrets[name]; !ok {
		return nil, status.Errorf(codes.NotFound, "secret %q not found", name)
	}
	return &secretmanagerpb.Secret{Name: m.secretResourceName(name)}, nil
}

func (m *mockServer) CreateSecret(_ context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error) {
	name := req.SecretId
	m.secrets[name] = ""
	return &secretmanagerpb.Secret{Name: m.secretResourceName(name)}, nil
}

func (m *mockServer) AddSecretVersion(_ context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	name := shortName(req.Parent)
	if _, ok := m.secrets[name]; !ok {
		return nil, status.Errorf(codes.NotFound, "secret %q not found", name)
	}
	val := string(req.Payload.Data)
	m.secrets[name] = val
	m.versions[name] = append(m.versions[name], val)
	return &secretmanagerpb.SecretVersion{Name: m.secretResourceName(name) + "/versions/latest"}, nil
}

func (m *mockServer) AccessSecretVersion(_ context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	// Strip "/versions/latest" suffix.
	secretPath := req.Name
	if idx := lastIndex(secretPath, "/versions/"); idx >= 0 {
		secretPath = secretPath[:idx]
	}
	name := shortName(secretPath)
	val, ok := m.secrets[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "secret %q not found", name)
	}
	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{Data: []byte(val)},
	}, nil
}

func (m *mockServer) DeleteSecret(_ context.Context, req *secretmanagerpb.DeleteSecretRequest) (*emptypb.Empty, error) {
	name := shortName(req.Name)
	if _, ok := m.secrets[name]; !ok {
		return nil, status.Errorf(codes.NotFound, "secret %q not found", name)
	}
	delete(m.secrets, name)
	delete(m.versions, name)
	return &emptypb.Empty{}, nil
}

func (m *mockServer) ListSecrets(_ context.Context, req *secretmanagerpb.ListSecretsRequest) (*secretmanagerpb.ListSecretsResponse, error) {
	// Filter is "name:{prefix}" — extract the prefix.
	filterPrefix := ""
	if req.Filter != "" {
		const pfx = "name:"
		if len(req.Filter) > len(pfx) {
			filterPrefix = req.Filter[len(pfx):]
		}
	}
	var secrets []*secretmanagerpb.Secret
	for name := range m.secrets {
		if filterPrefix == "" || startsWith(name, filterPrefix) {
			secrets = append(secrets, &secretmanagerpb.Secret{Name: m.secretResourceName(name)})
		}
	}
	return &secretmanagerpb.ListSecretsResponse{Secrets: secrets}, nil
}

// shortName extracts the last path segment (the secret short name).
func shortName(fullPath string) string {
	for i := len(fullPath) - 1; i >= 0; i-- {
		if fullPath[i] == '/' {
			return fullPath[i+1:]
		}
	}
	return fullPath
}

func lastIndex(s, substr string) int {
	idx := -1
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			idx = i
		}
	}
	return idx
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// newTestClient wires up a bufconn gRPC server and returns a *gcp.Client.
func newTestClient(t *testing.T) *gcp.Client {
	t.Helper()

	mock := newMockServer()

	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)

	grpcSrv := grpc.NewServer()
	secretmanagerpb.RegisterSecretManagerServiceServer(grpcSrv, mock)
	go grpcSrv.Serve(lis) //nolint:errcheck
	t.Cleanup(grpcSrv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	smClient, err := secretmanager.NewClient(t.Context(),
		option.WithGRPCConn(conn),
		option.WithoutAuthentication(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { smClient.Close() })

	return gcp.NewWithClient(smClient, testProject)
}

func TestClient_Get(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx := t.Context()

	t.Run("returns value for existing secret", func(t *testing.T) {
		t.Parallel()
		val, err := client.Get(ctx, testPath)
		require.NoError(t, err)
		assert.Equal(t, testValue, val)
	})

	t.Run("returns ErrNotFound for missing secret", func(t *testing.T) {
		t.Parallel()
		_, err := client.Get(ctx, provider.SecretPath{Env: testEnv, App: testApp, Key: "missing"})
		require.ErrorIs(t, err, provider.ErrNotFound)
	})
}

func TestClient_Set(t *testing.T) {
	t.Parallel()

	t.Run("creates new secret", func(t *testing.T) {
		t.Parallel()
		client := newTestClient(t)
		err := client.Set(t.Context(), provider.SecretPath{Env: testEnv, App: testApp, Key: "newkey"}, "newvalue")
		require.NoError(t, err)
	})

	t.Run("updates existing secret", func(t *testing.T) {
		t.Parallel()
		client := newTestClient(t)
		err := client.Set(t.Context(), testPath, "updated")
		require.NoError(t, err)

		val, err := client.Get(t.Context(), testPath)
		require.NoError(t, err)
		assert.Equal(t, "updated", val)
	})
}

func TestClient_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing secret", func(t *testing.T) {
		t.Parallel()
		client := newTestClient(t)
		err := client.Delete(t.Context(), testPath)
		require.NoError(t, err)
	})

	t.Run("idempotent on missing secret", func(t *testing.T) {
		t.Parallel()
		client := newTestClient(t)
		err := client.Delete(t.Context(), provider.SecretPath{Env: testEnv, App: testApp, Key: "missing"})
		require.NoError(t, err)
	})
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)

	// Add a second key so we have something to list alongside db_password.
	require.NoError(t, client.Set(t.Context(), provider.SecretPath{Env: testEnv, App: testApp, Key: "api_key"}, "apivalue"))

	keys, err := client.List(t.Context(), provider.SecretPath{Env: testEnv, App: testApp})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{testKey, "api_key"}, keys)
}

func TestClient_Exists(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx := t.Context()

	t.Run("true for existing secret", func(t *testing.T) {
		t.Parallel()
		ok, err := client.Exists(ctx, testPath)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("false for missing secret", func(t *testing.T) {
		t.Parallel()
		ok, err := client.Exists(ctx, provider.SecretPath{Env: testEnv, App: testApp, Key: "missing"})
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
