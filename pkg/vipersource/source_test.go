package vipersource_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stlimtat/kumokagi/pkg/vipersource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	secrets map[string]string
}

func (m *mockProvider) Get(_ context.Context, path provider.SecretPath) (string, error) {
	val, ok := m.secrets[path.Key]
	if !ok {
		return "", provider.ErrNotFound
	}
	return val, nil
}

func (m *mockProvider) Set(_ context.Context, _ provider.SecretPath, _ string) error { return nil }
func (m *mockProvider) Delete(_ context.Context, _ provider.SecretPath) error        { return nil }
func (m *mockProvider) List(_ context.Context, _ provider.SecretPath) ([]string, error) {
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}
func (m *mockProvider) Exists(_ context.Context, path provider.SecretPath) (bool, error) {
	_, ok := m.secrets[path.Key]
	return ok, nil
}

func makeConfig(keys []string) *config.Config {
	return &config.Config{
		Mount: "secret", Env: "prod", App: "myapp",
		Keys: keys,
	}
}

func TestSource_Load(t *testing.T) {
	t.Parallel()
	p := &mockProvider{secrets: map[string]string{
		"db_password": "s3cr3t",
		"api_key":     "abc123",
	}}
	src := vipersource.New(p, makeConfig([]string{"db_password", "api_key"}))

	v := viper.New()
	require.NoError(t, src.Load(t.Context(), v))

	assert.Equal(t, "s3cr3t", v.GetString("db_password"))
	assert.Equal(t, "abc123", v.GetString("api_key"))
}

func TestSource_Load_MissingSecret(t *testing.T) {
	t.Parallel()
	p := &mockProvider{secrets: map[string]string{"db_password": "s3cr3t"}}
	src := vipersource.New(p, makeConfig([]string{"db_password", "missing_key"}))

	err := src.Load(t.Context(), viper.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, provider.ErrNotFound)
}

func TestSource_Verify_AllPresent(t *testing.T) {
	t.Parallel()
	p := &mockProvider{secrets: map[string]string{
		"db_password": "s3cr3t",
		"api_key":     "abc123",
	}}
	src := vipersource.New(p, makeConfig([]string{"db_password", "api_key"}))
	require.NoError(t, src.Verify(t.Context()))
}

func TestSource_Verify_MissingKey(t *testing.T) {
	t.Parallel()
	p := &mockProvider{secrets: map[string]string{"db_password": "s3cr3t"}}
	src := vipersource.New(p, makeConfig([]string{"db_password", "api_key"}))

	err := src.Verify(t.Context())
	require.ErrorContains(t, err, "api_key")
}

func TestSource_Prune_ReturnsOrphans(t *testing.T) {
	t.Parallel()
	// Backend has db_password + api_key + old_key; config declares only db_password + api_key
	p := &mockProvider{secrets: map[string]string{
		"db_password": "s3cr3t",
		"api_key":     "abc123",
		"old_key":     "stale",
	}}
	src := vipersource.New(p, makeConfig([]string{"db_password", "api_key"}))

	orphans, err := src.Prune(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"old_key"}, orphans)
}

func TestSource_Prune_NoOrphans(t *testing.T) {
	t.Parallel()
	p := &mockProvider{secrets: map[string]string{
		"db_password": "s3cr3t",
		"api_key":     "abc123",
	}}
	src := vipersource.New(p, makeConfig([]string{"db_password", "api_key"}))

	orphans, err := src.Prune(t.Context())
	require.NoError(t, err)
	assert.Empty(t, orphans)
}
