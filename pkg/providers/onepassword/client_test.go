package onepassword_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stlimtat/kumokagi/pkg/provider"
	"github.com/stlimtat/kumokagi/pkg/providers/onepassword"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// mockExec returns a fake execCmd function. responses maps a substring of the
// joined args to (output, error). The first matching key wins.
type mockResponse struct {
	output string
	err    error
}

func newMockClient(vault string, responses map[string]mockResponse) *onepassword.Client {
	return onepassword.NewWithExec(vault, func(_ context.Context, _ string, args ...string) ([]byte, error) {
		key := fmt.Sprintf("%v", args)
		for pattern, resp := range responses {
			if containsAll(key, pattern) {
				if resp.err != nil {
					return nil, resp.err
				}
				return []byte(resp.output), nil
			}
		}
		// Default: not found
		return nil, fmt.Errorf("op: item not found")
	})
}

// containsAll checks that all space-separated tokens in pattern appear in s.
func containsAll(s, pattern string) bool {
	for _, tok := range splitTokens(pattern) {
		if !contains(s, tok) {
			return false
		}
	}
	return true
}

func splitTokens(s string) []string {
	var tokens []string
	for _, t := range splitOn(s, ' ') {
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

func splitOn(s string, sep rune) []string {
	var parts []string
	var cur []byte
	for _, r := range s {
		if r == sep {
			parts = append(parts, string(cur))
			cur = cur[:0]
		} else {
			cur = append(cur, byte(r))
		}
	}
	return append(parts, string(cur))
}

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

var testPath = provider.SecretPath{
	Mount: "Development",
	Env:   "prod",
	App:   "myapp",
	Key:   "db_password",
}

const itemTitle = "prod--myapp"

func TestGet_Found(t *testing.T) {
	t.Parallel()
	c := newMockClient("Development", map[string]mockResponse{
		"op read": {output: "s3cr3t"},
	})
	val, err := c.Get(context.Background(), testPath)
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t", val)
}

func TestGet_NotFound(t *testing.T) {
	t.Parallel()
	c := newMockClient("Development", map[string]mockResponse{})
	_, err := c.Get(context.Background(), testPath)
	require.ErrorIs(t, err, provider.ErrNotFound)
}

func TestSet_Create(t *testing.T) {
	t.Parallel()
	// item get fails → create path
	calls := &[]string{}
	c := onepassword.NewWithExec("Development", func(_ context.Context, _ string, args ...string) ([]byte, error) {
		key := fmt.Sprintf("%v", args)
		*calls = append(*calls, key)
		if containsAll(key, "item get") {
			return nil, fmt.Errorf("not found")
		}
		// item create succeeds
		return []byte("{}"), nil
	})
	err := c.Set(context.Background(), testPath, "newvalue")
	require.NoError(t, err)
	// Verify create was called
	found := false
	for _, call := range *calls {
		if containsAll(call, "item create") {
			found = true
		}
	}
	assert.True(t, found, "expected item create to be called")
}

func TestSet_Update(t *testing.T) {
	t.Parallel()
	calls := &[]string{}
	c := onepassword.NewWithExec("Development", func(_ context.Context, _ string, args ...string) ([]byte, error) {
		key := fmt.Sprintf("%v", args)
		*calls = append(*calls, key)
		// item get succeeds → update path
		return []byte(`{"fields":[]}`), nil
	})
	err := c.Set(context.Background(), testPath, "updated")
	require.NoError(t, err)
	found := false
	for _, call := range *calls {
		if containsAll(call, "item edit") {
			found = true
		}
	}
	assert.True(t, found, "expected item edit to be called")
}

func TestDelete_Found(t *testing.T) {
	t.Parallel()
	calls := &[]string{}
	c := onepassword.NewWithExec("Development", func(_ context.Context, _ string, args ...string) ([]byte, error) {
		key := fmt.Sprintf("%v", args)
		*calls = append(*calls, key)
		return []byte(`{"fields":[]}`), nil
	})
	err := c.Delete(context.Background(), testPath)
	require.NoError(t, err)
	found := false
	for _, call := range *calls {
		if containsAll(call, "item edit") && containsAll(call, "[delete]") {
			found = true
		}
	}
	assert.True(t, found, "expected field delete edit to be called")
}

func TestDelete_ItemNotFound(t *testing.T) {
	t.Parallel()
	// item get fails → idempotent, no error
	c := newMockClient("Development", map[string]mockResponse{})
	err := c.Delete(context.Background(), testPath)
	require.NoError(t, err)
}

func TestList(t *testing.T) {
	t.Parallel()
	itemJSON := `{"fields":[
		{"label":"db_password"},
		{"label":"api_key"},
		{"label":"notesPlain"},
		{"label":"password"}
	]}`
	c := newMockClient("Development", map[string]mockResponse{
		"item get": {output: itemJSON},
	})
	keys, err := c.List(context.Background(), testPath)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"db_password", "api_key"}, keys)
}

func TestList_ItemNotFound(t *testing.T) {
	t.Parallel()
	c := newMockClient("Development", map[string]mockResponse{})
	keys, err := c.List(context.Background(), testPath)
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestExists_True(t *testing.T) {
	t.Parallel()
	c := newMockClient("Development", map[string]mockResponse{
		"item get": {output: `[{"label":"db_password","value":"s3cr3t"}]`},
	})
	ok, err := c.Exists(context.Background(), testPath)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestExists_False(t *testing.T) {
	t.Parallel()
	c := newMockClient("Development", map[string]mockResponse{})
	ok, err := c.Exists(context.Background(), testPath)
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestOpArgs_EndOfOptionsTerminator verifies every op invocation places a "--"
// end-of-options terminator before untrusted operands (item title, field
// assignment, secret ref), so an operand can never be parsed as an op flag.
func TestOpArgs_EndOfOptionsTerminator(t *testing.T) {
	t.Parallel()

	// dashDashBeforeOperand returns true if "--" appears and the operand
	// (the last positional) follows it.
	assertTerminated := func(t *testing.T, args []string) {
		t.Helper()
		idx := -1
		for i, a := range args {
			if a == "--" {
				idx = i
			}
		}
		require.NotEqual(t, -1, idx, "expected a -- terminator in args: %v", args)
		assert.Less(t, idx, len(args)-1, "expected an operand after -- in args: %v", args)
		for _, a := range args[idx+1:] {
			assert.NotEqual(t, "-", string(a[0]), "operand after -- must not look like a flag: %q", a)
		}
	}

	var captured [][]string
	c := onepassword.NewWithExec("Development", func(_ context.Context, _ string, args ...string) ([]byte, error) {
		captured = append(captured, args)
		key := fmt.Sprintf("%v", args)
		if containsAll(key, "item get") {
			return []byte(`{"fields":[]}`), nil
		}
		return []byte("s3cr3t"), nil
	})

	ctx := context.Background()
	_, _ = c.Get(ctx, testPath)
	_ = c.Set(ctx, testPath, "v")
	_ = c.Delete(ctx, testPath)
	_, _ = c.List(ctx, testPath)
	_, _ = c.Exists(ctx, testPath)

	require.NotEmpty(t, captured)
	for _, args := range captured {
		assertTerminated(t, args)
	}
}
