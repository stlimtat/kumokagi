package onepassword

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/stlimtat/kumokagi/pkg/config"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

// Client implements provider.Provider using the op CLI.
// Assumes `op signin` has been completed and op is in PATH.
type Client struct {
	vault   string // 1Password vault name (from cfg.Mount)
	execCmd func(ctx context.Context, name string, args ...string) ([]byte, error)
}

// New creates a 1Password CLI provider. op must be in PATH and signed in.
func New(cfg *config.Config) *Client {
	return &Client{
		vault:   cfg.Mount,
		execCmd: runCmd,
	}
}

// NewWithExec creates a Client with an injected exec function (for testing).
func NewWithExec(vault string, execFn func(ctx context.Context, name string, args ...string) ([]byte, error)) *Client {
	return &Client{vault: vault, execCmd: execFn}
}

// runCmd is the real exec implementation.
func runCmd(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output() //nolint:gosec
}

func (c *Client) itemTitle(path provider.SecretPath) string {
	return fmt.Sprintf("%s--%s", path.Env, path.App)
}

// Get fetches a field value using op read.
func (c *Client) Get(ctx context.Context, path provider.SecretPath) (string, error) {
	ref := fmt.Sprintf("op://%s/%s/%s", c.vault, c.itemTitle(path), path.Key)
	out, err := c.execCmd(ctx, "op", "read", ref, "--no-newline")
	if err != nil {
		// op exits non-zero when the item/field doesn't exist
		return "", provider.ErrNotFound
	}
	return string(out), nil
}

// Set creates or updates a field in the 1Password item.
func (c *Client) Set(ctx context.Context, path provider.SecretPath, value string) error {
	title := c.itemTitle(path)
	field := fmt.Sprintf("%s=%s", path.Key, value)

	// Check if item exists
	_, err := c.execCmd(ctx, "op", "item", "get", title, "--vault="+c.vault, "--format=json")
	if err != nil {
		// Item doesn't exist — create it
		_, err = c.execCmd(ctx, "op", "item", "create",
			"--vault="+c.vault,
			"--title="+title,
			"--category=Login",
			field,
		)
		if err != nil {
			return fmt.Errorf("op create %s/%s: %w", c.vault, title, err)
		}
		return nil
	}

	// Item exists — edit the field
	_, err = c.execCmd(ctx, "op", "item", "edit", title, "--vault="+c.vault, field)
	if err != nil {
		return fmt.Errorf("op edit %s/%s: %w", c.vault, title, err)
	}
	return nil
}

// Delete removes a field from the 1Password item (idempotent).
func (c *Client) Delete(ctx context.Context, path provider.SecretPath) error {
	title := c.itemTitle(path)

	// Check if item exists first
	_, err := c.execCmd(ctx, "op", "item", "get", title, "--vault="+c.vault, "--format=json")
	if err != nil {
		// Item not found — nothing to delete
		return nil
	}

	// ponytail: op CLI v2 field-level delete syntax
	_, err = c.execCmd(ctx, "op", "item", "edit", title, "--vault="+c.vault,
		fmt.Sprintf("%s[delete]", path.Key),
	)
	if err != nil {
		return fmt.Errorf("op delete field %s/%s/%s: %w", c.vault, title, path.Key, err)
	}
	return nil
}

// opItem is a minimal struct for parsing op item JSON output.
type opItem struct {
	Fields []opField `json:"fields"`
}

type opField struct {
	Label string `json:"label"`
}

// systemFields are built-in 1Password field labels to exclude from List results.
var systemFields = map[string]bool{
	"notesPlain": true,
	"password":   true,
	"username":   true,
}

// List returns all user-defined field labels for the env+app item.
func (c *Client) List(ctx context.Context, path provider.SecretPath) ([]string, error) {
	title := c.itemTitle(path)
	out, err := c.execCmd(ctx, "op", "item", "get", title, "--vault="+c.vault, "--format=json")
	if err != nil {
		// Item not found — return empty list
		return []string{}, nil
	}

	var item opItem
	if err := json.Unmarshal(out, &item); err != nil {
		return nil, fmt.Errorf("op list parse %s/%s: %w", c.vault, title, err)
	}

	labels := make([]string, 0, len(item.Fields))
	for _, f := range item.Fields {
		if !systemFields[f.Label] && strings.TrimSpace(f.Label) != "" {
			labels = append(labels, f.Label)
		}
	}
	return labels, nil
}

// Exists returns true if the field exists in the 1Password item.
func (c *Client) Exists(ctx context.Context, path provider.SecretPath) (bool, error) {
	title := c.itemTitle(path)
	_, err := c.execCmd(ctx, "op", "item", "get", title,
		"--vault="+c.vault,
		"--fields", "label="+path.Key,
		"--format=json",
	)
	return err == nil, nil
}
