package factory

import (
	"context"
	"fmt"

	"github.com/stlimtat/kumokagi/internal/aws"
	"github.com/stlimtat/kumokagi/internal/azure"
	"github.com/stlimtat/kumokagi/internal/gcp"
	"github.com/stlimtat/kumokagi/internal/onepassword"
	"github.com/stlimtat/kumokagi/internal/vault"
	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// New returns the Provider for the backend named in cfg.Backend.
func New(ctx context.Context, cfg *config.Config) (provider.Provider, error) {
	switch cfg.Backend {
	case "vault":
		return vault.New(ctx)
	case "aws":
		return aws.New(ctx, cfg)
	case "azure":
		return azure.New(ctx, cfg)
	case "gcp":
		return gcp.New(ctx, cfg)
	case "onepassword":
		return onepassword.New(cfg), nil
	default:
		return nil, fmt.Errorf("unknown backend %q (valid: vault, aws, azure, gcp, onepassword)", cfg.Backend)
	}
}
