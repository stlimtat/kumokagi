package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stlimtat/kumokagi/pkg/provider"
)

var pruneConfirm bool

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "List secrets in the backend not declared in config (orphans). Use --confirm to delete.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		if err := loadConfig(ctx); err != nil {
			return err
		}
		orphaned, err := source.Prune(ctx)
		if err != nil {
			return err
		}
		if len(orphaned) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No orphaned secrets found")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Orphaned secrets (%d):\n", len(orphaned))
		for _, k := range orphaned {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s/%s/%s/%s\n",
				appCfg.Mount, appCfg.Env, appCfg.App, k)
		}
		if !pruneConfirm {
			fmt.Fprintln(cmd.OutOrStdout(), "\nDry run. Pass --confirm to delete.")
			return nil
		}
		for _, k := range orphaned {
			if err := vaultClient.Delete(ctx, provider.SecretPath{
				Mount: appCfg.Mount,
				Env:   appCfg.Env,
				App:   appCfg.App,
				Key:   k,
			}); err != nil {
				return fmt.Errorf("delete %s: %w", k, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", k)
		}
		return nil
	},
}

func init() {
	pruneCmd.Flags().BoolVar(&pruneConfirm, "confirm", false, "actually delete orphaned secrets")
	rootCmd.AddCommand(pruneCmd)
}
