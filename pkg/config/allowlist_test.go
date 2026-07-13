package config_test

import (
	"testing"

	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckHostAllowed(t *testing.T) {
	const env = config.EnvAllowedVaultAddrs

	// Unset allowlist permits anything (opt-in).
	t.Setenv(env, "")
	require.NoError(t, config.CheckHostAllowed(env, "https://attacker.example.com"))

	// Host match, tolerant of scheme/path and given as bare host or full URL.
	t.Setenv(env, "https://vault.corp.example.com, vault2.corp.example.com")
	require.NoError(t, config.CheckHostAllowed(env, "https://vault.corp.example.com"))
	require.NoError(t, config.CheckHostAllowed(env, "https://vault2.corp.example.com/v1"))
	require.NoError(t, config.CheckHostAllowed(env, "https://VAULT.corp.example.com")) // case-insensitive

	// A redirected host fails closed.
	assert.Error(t, config.CheckHostAllowed(env, "https://attacker.example.com"))
	assert.Error(t, config.CheckHostAllowed(env, "https://vault.corp.example.com.attacker.com"))
}

func TestCheckValueAllowed(t *testing.T) {
	const env = config.EnvAllowedGCPProjects
	t.Setenv(env, "")
	require.NoError(t, config.CheckValueAllowed(env, "any-project"))

	t.Setenv(env, "prod-secrets, staging-secrets")
	require.NoError(t, config.CheckValueAllowed(env, "prod-secrets"))
	assert.Error(t, config.CheckValueAllowed(env, "attacker-project"))
}

func TestValidate_OnePasswordVaultSlash(t *testing.T) {
	cfg := &config.Config{Backend: "onepassword", Mount: "Team/evil", App: "myapp", Env: "prod"}
	assert.ErrorContains(t, cfg.Validate(), "must not contain '/'")
}
