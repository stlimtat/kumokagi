package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInit_CreatesConfigFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"init", "--app", "testapp", "--env", "staging"})
	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".kumokagi.yaml"))
	assert.Contains(t, buf.String(), "testapp")
}

func TestRunInit_ErrorsIfFileExists(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(origDir) })

	// Create the file first
	require.NoError(t, os.WriteFile(".kumokagi.yaml", []byte("existing"), 0o600))

	rootCmd.SetArgs([]string{"init", "--app", "testapp"})
	err := rootCmd.Execute()
	require.Error(t, err)
}
