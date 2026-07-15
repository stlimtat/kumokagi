package factory_test

import (
	"context"
	"testing"

	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/factory"
	_ "github.com/stlimtat/kumokagi/pkg/factory/all"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllBackendsRegistered guards against a provider forgetting its init()
// factory.Register — the Vault provider shipped unregistered, so `--backend
// vault` returned "unknown backend". Each backend must at least resolve to a
// constructor (construction itself may still fail without live credentials).
func TestAllBackendsRegistered(t *testing.T) {
	cases := map[string]*config.Config{
		"vault":       {Backend: "vault", App: "a", Env: "prod", Mount: "secret", Vault: config.VaultConfig{Address: "https://vault.example.com"}},
		"onepassword": {Backend: "onepassword", App: "a", Env: "prod", Mount: "Private"},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := factory.New(context.Background(), cfg)
			require.NoError(t, err)
		})
	}
}

func TestNew_UnknownBackend(t *testing.T) {
	_, err := factory.New(context.Background(), &config.Config{Backend: "nope", App: "a", Env: "prod"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown backend")
}
