# env — Environment variable management for Go

![CI](https://github.com/go-slim/env/actions/workflows/ci.yml/badge.svg)

A small, dependency-light helper to load and query environment variables, with:

- Scoped lookups via a Signer: `PREFIX` and optional `CATEGORY` (e.g. `CACHE_BOOK_*` falling back to `CACHE_*`).
- Simple typed getters: `String`, `Bytes`, `Int`, `Bool`, `Duration`, `List`.
- Struct filling via tags: `s.Fill(&cfg)` with `env:"KEY"` tags.
- Global helpers and `.env` loader chain: `.env`, `.env.local`, `.env.<APP_ENV>`, `.env.<APP_ENV>.local`.

Module path: `go-slim.dev/env`

## Install

```bash
go get go-slim.dev/env
```

Go version: `1.22` (per `env/go.mod`).

## Quick Start

```go
package main

import (
    "fmt"
    env "go-slim.dev/env"
)

func main() {
    // Initialize from current directory (loads .env files if present)
    _ = env.Init() // ignore error if no .env files exist

    // Signed lookups group by prefix and optional category.
    // Keys are matched as: PREFIX_CATEGORY_KEY first, then fallback to PREFIX_KEY.
    cache := env.Signed("CACHE", "BOOK")

    // Prefer category-level
    driver := cache.String("DRIVER")         // from CACHE_BOOK_DRIVER (or "")
    // Fallback to prefix-level
    database := cache.Int("DATABASE")        // from CACHE_BOOK_DATABASE else CACHE_DATABASE

    fmt.Println(driver, database)
}
```

## Signed lookups

Given environment variables like:

```
CACHE_DRIVER=redis
CACHE_DATABASE=1
CACHE_SCOPE=app:
CACHE_BOOK_DATABASE=10
CACHE_BOOK_SCOPE=app:books:
```

You can scope queries by category:

```go
cache := env.Signed("CACHE", "BOOK")
_ = cache.String("DRIVER")   // "redis" (fallback from CACHE_DRIVER)
_ = cache.Int("DATABASE")     // 10 (from CACHE_BOOK_DATABASE)
_ = cache.String("SCOPE")     // "app:books:" (from CACHE_BOOK_SCOPE)
```

Internally, the resolver tries `PREFIX_CATEGORY_KEY` first; if missing or empty, it falls back to `PREFIX_KEY`.

## Global helpers

- `env.Init(root ...string)` and `env.InitWithDir(dir string)` load:
  - `.env`, `.env.local`, `.env.<APP_ENV>`, `.env.<APP_ENV>.local`
  - The default `APP_ENV` is `prod` (also exposed via `env.String("APP_ENV")`).
- `env.Path(...)` returns the initialization root (with optional path join).
- `env.Is("dev", "prod", ...)` checks if `APP_ENV` matches any provided value.
- `env.All()` returns all loaded key-value pairs.
- `env.Load(files...)` reads extra `.env` files and merges values.
- `env.Inject(env.Environ, map[string]string)` injects in-memory values into an `Environ`.

All the above have instance-level equivalents on `env.Environ`.

## Typed getters

On both `Signer` and `Environ`:

- `String(key, ...fallback)`
- `Bytes(key, ...fallback)`
- `Int(key, ...fallback)`
- `Bool(key, ...fallback)`
- `Duration(key, ...fallback)` — parses Go duration strings (e.g. `"150ms"`, `"2s"`) or integers as nanoseconds.
- `List(key, ...fallback)` — splits on comma and trims spaces.

## Struct filling

Use `env:"KEY"` tags to fill struct fields from environment variables:

```go
type Feature struct {
    Flag bool `env:"FEATURE_FLAG"`
}

type Config struct {
    Host    string  `env:"HOST"`
    Port    int     `env:"PORT"`
    Debug   bool    `env:"DEBUG"`
    Timeout string  `env:"TIMEOUT"` // store as string; parse as time.Duration if needed
    Rate    float64 `env:"RATE"`
    Feat    Feature             // nested struct supported
    Opt     *Feature            // pointer-to-struct supported (if non-nil)
}

func load() (*Config, error) {
    _ = env.Init() // or InitWithDir
    cfg := &Config{Opt: &Feature{}}
    if err := env.Signed("APP", "WEB").Fill(cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

Notes:
- The filler leverages `reflect` and a light casting helper.
- If you need `time.Duration`, keep it as `string` in the struct and parse via `time.ParseDuration`.

## Iteration, Map, and Where

- `Environ.Map(prefix)` collects keys with the given prefix and returns a map with the prefix trimmed.
- `Environ.Where(func(name, value string) bool)` filters all key-value pairs.
- Under the hood, `Signer` also filters iteration by `PREFIX_CATEGORY_` and buffers `PREFIX_` values for fallback.

## Testing

From the module directory `env/`:

```bash
go test -v ./
# or race-enabled
go test -race -v ./
```

If running from a multi-module repo root with a `go.work`, you may prefer testing each module explicitly:

```bash
go test -v ./env
```

The test suite covers:
- Signed lookup and fallback semantics
- Iteration, buffering, and trimming
- Typed getters and list parsing
- Struct `Fill()` including nested/ptr structs
- Global helpers: `Init`, `Path`, `Is`, `All`, `Load`, `Inject`

## License

MIT
