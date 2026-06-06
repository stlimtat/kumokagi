package provider_test

import (
	"testing"

	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stretchr/testify/assert"
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
