package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

var (
	rotateLength int
	rotateShow   bool
)

var rotateCmd = &cobra.Command{
	Use:   "rotate <key>",
	Short: "Generate and store a new random secret value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if rotateLength < 16 || rotateLength > 4096 {
			return fmt.Errorf("--length must be between 16 and 4096 (got %d)", rotateLength)
		}
		if err := loadConfig(ctx); err != nil {
			return err
		}
		buf := make([]byte, rotateLength)
		if _, err := rand.Read(buf); err != nil {
			return fmt.Errorf("generate random bytes: %w", err)
		}
		// base64url, no padding — safe for all backends
		value := base64.RawURLEncoding.EncodeToString(buf)
		path := provider.SecretPath{
			Mount: appCfg.Mount,
			Env:   appCfg.Env,
			App:   appCfg.App,
			Key:   args[0],
		}
		if err := secretClient.Set(ctx, path, value); err != nil {
			return err
		}
		if rotateShow {
			fmt.Fprintln(cmd.OutOrStdout(), value)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Rotated %s/%s/%s/%s\n",
				appCfg.Mount, appCfg.Env, appCfg.App, args[0])
		}
		return nil
	},
}

func init() {
	rotateCmd.Flags().IntVar(&rotateLength, "length", 32, "number of random bytes before base64 encoding")
	rotateCmd.Flags().BoolVar(&rotateShow, "show", false, "print the new value to stdout")
	rootCmd.AddCommand(rotateCmd)
}
