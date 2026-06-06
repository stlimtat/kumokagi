package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		check   func(t *testing.T, cfg *config.Config)
	}{
		{
			name: "full config",
			yaml: `
backend: vault
mount: secret
app: myapp
env: prod
keys:
  - db_password
  - api_key
vault:
  address: https://vault.example.com
`,
			check: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "vault", cfg.Backend)
				assert.Equal(t, "secret", cfg.Mount)
				assert.Equal(t, "myapp", cfg.App)
				assert.Equal(t, "prod", cfg.Env)
				assert.Equal(t, []string{"db_password", "api_key"}, cfg.Keys)
				assert.Equal(t, "https://vault.example.com", cfg.Vault.Address)
			},
		},
		{
			name: "mount defaults to secret when omitted",
			yaml: `
backend: vault
app: myapp
env: staging
`,
			check: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "secret", cfg.Mount)
			},
		},
		{
			name:    "invalid yaml",
			yaml:    "{\nbad yaml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := filepath.Join(t.TempDir(), ".kumokagi.yaml")
			require.NoError(t, os.WriteFile(f, []byte(tt.yaml), 0o600))

			cfg, err := config.Load(f)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := config.Load("/nonexistent/.kumokagi.yaml")
	require.Error(t, err)
}

func TestLoad_EnvOverridesConfigEnv(t *testing.T) {
	yaml := `
backend: vault
app: myapp
env: prod
`
	f := filepath.Join(t.TempDir(), ".kumokagi.yaml")
	require.NoError(t, os.WriteFile(f, []byte(yaml), 0o600))
	t.Setenv("KUMOKAGI_ENV", "staging")

	cfg, err := config.Load(f)
	require.NoError(t, err)
	assert.Equal(t, "staging", cfg.Env)
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr string
	}{
		{
			name:    "missing backend",
			cfg:     config.Config{App: "myapp", Env: "prod"},
			wantErr: "backend is required",
		},
		{
			name:    "missing app",
			cfg:     config.Config{Backend: "vault", Env: "prod"},
			wantErr: "app is required",
		},
		{
			name:    "missing env",
			cfg:     config.Config{Backend: "vault", App: "myapp"},
			wantErr: "env is required",
		},
		{
			name: "valid",
			cfg:  config.Config{Backend: "vault", App: "myapp", Env: "prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
