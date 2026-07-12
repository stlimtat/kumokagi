package provider_test

import (
	"context"
	"testing"

	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretPath_DataPath(t *testing.T) {
	t.Parallel()
	p := provider.SecretPath{Mount: "secret", Env: "prod", App: "myapp", Key: "db_password"}
	assert.Equal(t, "secret/data/prod/myapp/db_password", p.DataPath())
}

func TestSecretPath_MetadataPath(t *testing.T) {
	t.Parallel()
	p := provider.SecretPath{Mount: "secret", Env: "prod", App: "myapp", Key: "db_password"}
	assert.Equal(t, "secret/metadata/prod/myapp/db_password", p.MetadataPath())
}

func TestSecretPath_ListPath(t *testing.T) {
	t.Parallel()
	p := provider.SecretPath{Mount: "kv", Env: "staging", App: "api", Key: "ignored"}
	assert.Equal(t, "kv/metadata/staging/api", p.ListPath())
}

func TestErrNotFound(t *testing.T) {
	t.Parallel()
	assert.EqualError(t, provider.ErrNotFound, "secret not found")
}

func TestSecretPath_Validate(t *testing.T) {
	t.Parallel()
	valid := []provider.SecretPath{
		{Mount: "secret", Env: "prod", App: "myapp", Key: "db_password"},
		{Mount: "secret", Env: "prod", App: "myapp"},                                 // empty key (list path) is allowed
		{Mount: "https://x.vault.azure.net", Env: "prod", App: "my.app", Key: "k-1"}, // URL mount, dotted app
		{Mount: "", Env: "prod", App: "app", Key: "k"},                               // empty mount (AWS) is allowed
	}
	for _, p := range valid {
		assert.NoErrorf(t, p.Validate(), "expected %+v to be valid", p)
	}

	invalid := map[string]provider.SecretPath{
		"vault path traversal in key": {Mount: "secret", Env: "prod", App: "app", Key: "../../metadata/prod/other/k"},
		"slash in app":                {Mount: "secret", Env: "prod", App: "a/b", Key: "k"},
		"op option injection in key":  {Mount: "v", Env: "prod", App: "app", Key: "--vault=evil"},
		"assignment syntax in key":    {Mount: "v", Env: "prod", App: "app", Key: "password[password]"},
		"leading dash in env":         {Mount: "secret", Env: "-x", App: "app", Key: "k"},
		"traversal in mount":          {Mount: "secret/..", Env: "prod", App: "app", Key: "k"},
		"newline in app":              {Mount: "secret", Env: "prod", App: "app\nx", Key: "k"},
		"empty env":                   {Mount: "secret", Env: "", App: "app", Key: "k"},
	}
	for name, p := range invalid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, p.Validate(), provider.ErrInvalidPath)
		})
	}
}

func TestParseURI_RejectsTraversal(t *testing.T) {
	t.Parallel()
	// %2F decodes to "/", so without validation the key would traverse the
	// Vault logical path out of the env/app namespace.
	_, _, err := provider.ParseURI("kumokagi://vault/secret/prod/myapp/..%2F..%2Fmetadata%2Fprod%2Fother%2Fk")
	assert.ErrorIs(t, err, provider.ErrInvalidPath)
}

// stubProvider records the last path it was asked to Get.
type stubProvider struct{ got provider.SecretPath }

func (s *stubProvider) Get(_ context.Context, p provider.SecretPath) (string, error) {
	s.got = p
	return "value", nil
}
func (s *stubProvider) Set(context.Context, provider.SecretPath, string) error { return nil }
func (s *stubProvider) Delete(context.Context, provider.SecretPath) error      { return nil }
func (s *stubProvider) List(context.Context, provider.SecretPath) ([]string, error) {
	return nil, nil
}
func (s *stubProvider) Exists(context.Context, provider.SecretPath) (bool, error) { return true, nil }

func TestValidating_BlocksInjectionBeforeBackend(t *testing.T) {
	t.Parallel()
	stub := &stubProvider{}
	v := provider.NewValidating(stub)

	// A malicious key never reaches the wrapped provider.
	_, err := v.Get(context.Background(), provider.SecretPath{
		Mount: "secret", Env: "prod", App: "app", Key: "--vault=evil",
	})
	require.ErrorIs(t, err, provider.ErrInvalidPath)
	assert.Equal(t, provider.SecretPath{}, stub.got, "malicious path must not reach the backend")

	// A clean path passes through untouched.
	clean := provider.SecretPath{Mount: "secret", Env: "prod", App: "app", Key: "db_password"}
	val, err := v.Get(context.Background(), clean)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
	assert.Equal(t, clean, stub.got)
}
