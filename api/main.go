package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/config"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/lifecycle"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/datadir"
	"github.com/stanza-go/standalone/migration"
	"github.com/stanza-go/standalone/module/adminauth"
	"github.com/stanza-go/standalone/module/health"
	"github.com/stanza-go/standalone/seed"
)

func main() {
	app := lifecycle.New(
		lifecycle.Provide(provideDataDir),
		lifecycle.Provide(provideConfig),
		lifecycle.Provide(provideLogger),
		lifecycle.Provide(provideDB),
		lifecycle.Provide(provideAuth),
		lifecycle.Provide(provideRouter),
		lifecycle.Provide(provideServer),
		lifecycle.Invoke(registerModules),
	)

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func provideDataDir() (*datadir.Dir, error) {
	return datadir.Resolve()
}

func provideConfig(dir *datadir.Dir) *config.Config {
	cfg, err := config.Load(dir.Config,
		config.WithEnvPrefix("STANZA"),
		config.WithDefaults(map[string]string{
			"server.addr": ":23710",
			"log.level":   "info",
		}),
	)
	if err != nil {
		// Config file may not exist yet — fall back to defaults + env.
		cfg = config.New(
			config.WithEnvPrefix("STANZA"),
			config.WithDefaults(map[string]string{
				"server.addr": ":23710",
				"log.level":   "info",
			}),
		)
	}
	return cfg
}

func provideLogger(dir *datadir.Dir, cfg *config.Config) (*log.Logger, error) {
	level := log.ParseLevel(cfg.GetString("log.level"))

	fw, err := log.NewFileWriter(dir.Logs)
	if err != nil {
		return nil, fmt.Errorf("log file writer: %w", err)
	}

	logger := log.New(
		log.WithLevel(level),
		log.WithWriter(io.MultiWriter(os.Stdout, fw)),
	)

	return logger, nil
}

func provideDB(lc *lifecycle.Lifecycle, dir *datadir.Dir, logger *log.Logger) (*sqlite.DB, error) {
	db := sqlite.New(dir.DB)

	lc.Append(lifecycle.Hook{
		OnStart: func(ctx context.Context) error {
			if err := db.Start(ctx); err != nil {
				return fmt.Errorf("sqlite start: %w", err)
			}

			migration.Register(db)

			n, err := db.Migrate()
			if err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			if n > 0 {
				logger.Info("migrations applied",
					log.Int("count", n),
					log.String("backup", db.LastBackupPath()),
				)
			}

			if err := seed.Run(db, logger); err != nil {
				return fmt.Errorf("seed: %w", err)
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return db.Stop(ctx)
		},
	})

	return db, nil
}

func provideAuth(cfg *config.Config, logger *log.Logger) (*auth.Auth, error) {
	// Signing key from config or env (STANZA_AUTH_SIGNING_KEY).
	// If not set, generate a random key for development.
	keyHex := cfg.GetString("auth.signing_key")
	var signingKey []byte

	if keyHex != "" {
		var err error
		signingKey, err = hex.DecodeString(keyHex)
		if err != nil {
			return nil, fmt.Errorf("auth.signing_key: invalid hex: %w", err)
		}
		if len(signingKey) < 32 {
			return nil, fmt.Errorf("auth.signing_key: must be at least 32 bytes (64 hex chars)")
		}
	} else {
		signingKey = make([]byte, 32)
		if _, err := rand.Read(signingKey); err != nil {
			return nil, fmt.Errorf("generate signing key: %w", err)
		}
		logger.Warn("auth.signing_key not set — using random key (sessions won't survive restart)")
	}

	secureCookies := cfg.GetString("auth.secure_cookies") != "false"

	a := auth.New(signingKey,
		auth.WithSecureCookies(secureCookies),
	)

	return a, nil
}

func provideRouter(logger *log.Logger, cfg *config.Config) *http.Router {
	router := http.NewRouter()

	router.Use(http.RequestLogger(logger))

	// CORS: allow cross-origin requests from admin and UI dev servers.
	// Configure via STANZA_CORS_ORIGINS (comma-separated) or cors.origins
	// in config.yaml. Defaults to the admin and UI Vite dev servers.
	originsStr := cfg.GetStringOr("cors.origins", "http://localhost:23705,http://localhost:23700")
	if originsStr != "" {
		var origins []string
		for _, o := range strings.Split(originsStr, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				origins = append(origins, o)
			}
		}
		if len(origins) > 0 {
			router.Use(http.CORS(http.CORSConfig{
				AllowOrigins:     origins,
				AllowCredentials: true,
			}))
		}
	}

	router.Use(http.Recovery(func(v any, stack []byte) {
		logger.Error("panic recovered",
			log.Any("error", v),
			log.String("stack", string(stack)),
		)
	}))

	return router
}

func provideServer(lc *lifecycle.Lifecycle, router *http.Router, cfg *config.Config, logger *log.Logger) *http.Server {
	addr := cfg.GetStringOr("server.addr", ":23710")
	srv := http.NewServer(router, http.WithAddr(addr))

	lc.Append(lifecycle.Hook{
		OnStart: func(ctx context.Context) error {
			if err := srv.Start(ctx); err != nil {
				return err
			}
			logger.Info("server started", log.String("addr", srv.Addr()))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("server stopping")
			return srv.Stop(ctx)
		},
	})

	return srv
}

func registerModules(router *http.Router, db *sqlite.DB, a *auth.Auth, logger *log.Logger) {
	api := router.Group("/api")

	health.Register(api, db)
	adminauth.Register(api, a, db, logger)

	logger.Info("modules registered")
}
