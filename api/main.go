package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/config"
	"github.com/stanza-go/framework/pkg/cron"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/lifecycle"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/datadir"
	"github.com/stanza-go/standalone/migration"
	"github.com/stanza-go/standalone/module/adminaudit"
	"github.com/stanza-go/standalone/module/adminuploads"
	"github.com/stanza-go/standalone/module/adminauth"
	"github.com/stanza-go/standalone/module/admincron"
	"github.com/stanza-go/standalone/module/admindb"
	"github.com/stanza-go/standalone/module/adminlogs"
	"github.com/stanza-go/standalone/module/adminqueue"
	"github.com/stanza-go/standalone/module/adminsettings"
	"github.com/stanza-go/standalone/module/adminsessions"
	"github.com/stanza-go/standalone/module/adminusers"
	"github.com/stanza-go/standalone/module/dashboard"
	"github.com/stanza-go/standalone/module/health"
	"github.com/stanza-go/standalone/module/apikeys"
	"github.com/stanza-go/standalone/module/userauth"
	"github.com/stanza-go/standalone/module/userprofile"
	"github.com/stanza-go/standalone/module/usermgmt"
	"github.com/stanza-go/standalone/seed"
)

// signingKey holds the shared JWT signing key used by both admin and
// user auth instances.
type signingKey struct{ key []byte }

// userAuth wraps auth.Auth with user-specific cookie paths (/api
// instead of /api/admin). A distinct type so the DI container can
// provide both admin and user auth instances.
type userAuth struct{ *auth.Auth }

func main() {
	app := lifecycle.New(
		lifecycle.Provide(provideDataDir),
		lifecycle.Provide(provideConfig),
		lifecycle.Provide(provideLogger),
		lifecycle.Provide(provideDB),
		lifecycle.Provide(provideSigningKey),
		lifecycle.Provide(provideAuth),
		lifecycle.Provide(provideUserAuth),
		lifecycle.Provide(provideQueue),
		lifecycle.Provide(provideCron),
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

func provideSigningKey(cfg *config.Config, logger *log.Logger) (*signingKey, error) {
	// Signing key from config or env (STANZA_AUTH_SIGNING_KEY).
	// If not set, generate a random key for development.
	keyHex := cfg.GetString("auth.signing_key")
	var key []byte

	if keyHex != "" {
		var err error
		key, err = hex.DecodeString(keyHex)
		if err != nil {
			return nil, fmt.Errorf("auth.signing_key: invalid hex: %w", err)
		}
		if len(key) < 32 {
			return nil, fmt.Errorf("auth.signing_key: must be at least 32 bytes (64 hex chars)")
		}
	} else {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate signing key: %w", err)
		}
		logger.Warn("auth.signing_key not set — using random key (sessions won't survive restart)")
	}

	return &signingKey{key: key}, nil
}

func provideAuth(sk *signingKey, cfg *config.Config) *auth.Auth {
	secureCookies := cfg.GetString("auth.secure_cookies") != "false"
	return auth.New(sk.key, auth.WithSecureCookies(secureCookies))
}

func provideUserAuth(sk *signingKey, cfg *config.Config) *userAuth {
	secureCookies := cfg.GetString("auth.secure_cookies") != "false"
	return &userAuth{auth.New(sk.key,
		auth.WithCookiePath("/api"),
		auth.WithSecureCookies(secureCookies),
	)}
}

func provideQueue(lc *lifecycle.Lifecycle, db *sqlite.DB, logger *log.Logger) *queue.Queue {
	q := queue.New(db,
		queue.WithLogger(logger),
	)

	lc.Append(lifecycle.Hook{
		OnStart: q.Start,
		OnStop:  q.Stop,
	})

	return q
}

func provideCron(lc *lifecycle.Lifecycle, db *sqlite.DB, q *queue.Queue, logger *log.Logger) (*cron.Scheduler, error) {
	s := cron.NewScheduler(
		cron.WithLogger(logger),
		cron.WithOnComplete(func(r cron.CompletedRun) {
			errMsg := ""
			status := "success"
			if r.Err != nil {
				errMsg = r.Err.Error()
				status = "error"
			}
			_, err := db.Exec(
				"INSERT INTO cron_runs (name, started_at, duration_ms, status, error) VALUES (?, ?, ?, ?, ?)",
				r.Name,
				r.Started.UTC().Format(time.RFC3339),
				r.Duration.Milliseconds(),
				status,
				errMsg,
			)
			if err != nil {
				logger.Error("failed to persist cron run",
					log.String("job", r.Name),
					log.String("error", err.Error()),
				)
			}
		}),
	)

	// Purge completed/cancelled queue jobs older than 24h, every hour.
	if err := s.Add("purge-completed-jobs", "0 * * * *", func(ctx context.Context) error {
		n, err := q.Purge(24 * time.Hour)
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("purged old queue jobs", log.Int64("count", n))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-completed-jobs: %w", err)
	}

	// Purge expired refresh tokens every hour at :30.
	if err := s.Add("purge-expired-tokens", "30 * * * *", func(ctx context.Context) error {
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := db.Exec("DELETE FROM refresh_tokens WHERE expires_at < ?", now)
		if err != nil {
			return err
		}
		if res.RowsAffected > 0 {
			logger.Info("purged expired refresh tokens", log.Int64("count", res.RowsAffected))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-expired-tokens: %w", err)
	}

	// Purge revoked/expired API keys older than 30 days, daily at 3:00 AM.
	if err := s.Add("purge-stale-api-keys", "0 3 * * *", func(ctx context.Context) error {
		cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
		res, err := db.Exec(
			"DELETE FROM api_keys WHERE (revoked_at IS NOT NULL AND revoked_at < ?) OR (expires_at IS NOT NULL AND expires_at < ?)",
			cutoff, cutoff,
		)
		if err != nil {
			return err
		}
		if res.RowsAffected > 0 {
			logger.Info("purged stale API keys", log.Int64("count", res.RowsAffected))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-stale-api-keys: %w", err)
	}

	// Purge cron run history older than 7 days, daily at 3:30 AM.
	if err := s.Add("purge-old-cron-runs", "30 3 * * *", func(ctx context.Context) error {
		cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339)
		res, err := db.Exec("DELETE FROM cron_runs WHERE started_at < ?", cutoff)
		if err != nil {
			return err
		}
		if res.RowsAffected > 0 {
			logger.Info("purged old cron runs", log.Int64("count", res.RowsAffected))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-old-cron-runs: %w", err)
	}

	// Purge audit log entries older than 90 days, daily at 4:00 AM.
	if err := s.Add("purge-old-audit-log", "0 4 * * *", func(ctx context.Context) error {
		cutoff := time.Now().UTC().Add(-90 * 24 * time.Hour).Format(time.RFC3339)
		res, err := db.Exec("DELETE FROM audit_log WHERE created_at < ?", cutoff)
		if err != nil {
			return err
		}
		if res.RowsAffected > 0 {
			logger.Info("purged old audit log entries", log.Int64("count", res.RowsAffected))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-old-audit-log: %w", err)
	}

	lc.Append(lifecycle.Hook{
		OnStart: s.Start,
		OnStop:  s.Stop,
	})

	return s, nil
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

func registerModules(router *http.Router, db *sqlite.DB, a *auth.Auth, ua *userAuth, q *queue.Queue, s *cron.Scheduler, dir *datadir.Dir, logger *log.Logger) {
	api := router.Group("/api")

	// Public routes.
	health.Register(api, db)
	adminauth.Register(api, a, db, logger)
	userauth.Register(api, ua.Auth, db, logger)

	// Protected admin routes — require valid JWT + admin scope.
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))

	dashboard.Register(admin, db, q, s)
	adminusers.Register(admin, db)
	adminsessions.Register(admin, db)
	admincron.Register(admin, s, db)
	adminqueue.Register(admin, q, db)
	adminlogs.Register(admin, dir.Logs)
	admindb.Register(admin, db, dir.Backups)
	adminsettings.Register(admin, db)
	usermgmt.Register(admin, a, db)
	apikeys.Register(admin, db)
	adminaudit.Register(admin, db)
	adminuploads.Register(admin, db, dir.Uploads)

	// Protected user routes — require valid JWT + user scope.
	user := api.Group("/user")
	user.Use(ua.RequireAuth())
	user.Use(auth.RequireScope("user"))

	userprofile.Register(user, db, logger)

	// API key authenticated routes — for programmatic access.
	kv := apikeys.NewValidator(db)
	v1 := api.Group("/v1")
	v1.Use(auth.RequireAPIKey(kv))

	v1.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		http.WriteJSON(w, http.StatusOK, map[string]any{
			"uid":    claims.UID,
			"scopes": claims.Scopes,
		})
	})

	// Serve embedded frontend assets in production builds.
	adminFS := embeddedAdmin()
	uiFS := embeddedUI()

	if adminFS != nil {
		router.Handle("GET /admin/{path...}", http.Static(adminFS))
		logger.Info("serving embedded admin panel at /admin/")
	}
	if uiFS != nil {
		// Catch-all for unknown API GET routes — prevents the UI static
		// handler from serving index.html for /api/* requests.
		api.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
			http.WriteError(w, http.StatusNotFound, "not found")
		})
		router.Handle("GET /{path...}", http.Static(uiFS))
		logger.Info("serving embedded UI at /")
	}

	logger.Info("modules registered")
}
