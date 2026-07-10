---
aliases: ["/how-to/viper/"]
title: Go — Cobra/Viper
weight: 6
---

# Go — Cobra/Viper integration

kumokagi acts as a **source** in viper's config resolution chain ([ADR 0003]({{< relref "/adrs/0003-source-not-standalone" >}})): a cobra command wires it up once in `PersistentPreRunE`, and the rest of the application reads secrets through the same `viper.GetString()` calls it already uses for ordinary config. Secrets loaded via kumokagi take highest precedence (`v.Set()`), above env vars and config files.

![kumokagi architecture — where the viper source sits](/img/architecture.png)

## Setup

```go
package main

import (
    "context"
    "log"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "github.com/stlimtat/kumokagi/pkg/config"
    "github.com/stlimtat/kumokagi/pkg/factory"
    "github.com/stlimtat/kumokagi/pkg/vipersource"

    _ "github.com/stlimtat/kumokagi/pkg/factory/all" // register all backends
)

var rootCmd = &cobra.Command{
    PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
        ctx := cmd.Context()

        cfg, err := config.Load(".kumokagi.yaml")
        if err != nil {
            return err
        }
        provider, err := factory.New(ctx, cfg)
        if err != nil {
            return err
        }
        src := vipersource.New(provider, cfg)
        return src.Load(ctx, viper.GetViper())
    },
}
```

## Verify at startup

Check all declared secrets exist before starting the application:

```go
PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
    // ...
    src := vipersource.New(provider, cfg)
    if err := src.Verify(ctx); err != nil {
        return fmt.Errorf("missing secrets: %w", err)
    }
    return src.Load(ctx, viper.GetViper())
},
```

## Access secrets

```go
dbPassword := viper.GetString("db_password")
apiKey := viper.GetString("api_key")
```

## Connection pool rotation pattern

Because kumokagi never caches ([ADR 0001]({{< relref "/adrs/0001-no-in-memory-cache" >}})), re-loading after an auth failure always yields the rotated value:

```go
func newDBPool(ctx context.Context, src *vipersource.Source, v *viper.Viper) (*pgxpool.Pool, error) {
    password := v.GetString("db_password")
    return pgxpool.New(ctx, fmt.Sprintf("postgres://user:%s@host/db", password))
}

// On authentication failure, re-fetch and reconnect:
func getConn(ctx context.Context, pool *pgxpool.Pool, src *vipersource.Source, v *viper.Viper) (pgx.Conn, error) {
    conn, err := pool.Acquire(ctx)
    if err != nil {
        // Re-load secrets and rebuild pool
        src.Load(ctx, v)
        pool, _ = newDBPool(ctx, src, v)
        return pool.Acquire(ctx)
    }
    return conn, nil
}
```
