package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/cmd"
	"github.com/stanza-go/framework/pkg/config"
	"github.com/stanza-go/framework/pkg/cron"
	"github.com/stanza-go/framework/pkg/email"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/lifecycle"
	"github.com/stanza-go/framework/pkg/log"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/datadir"
	"github.com/stanza-go/standalone/migration"
	"github.com/stanza-go/standalone/module/adminaudit"
	"github.com/stanza-go/standalone/module/adminnotifications"
	"github.com/stanza-go/standalone/module/adminprofile"
	"github.com/stanza-go/standalone/module/adminwebhooks"
	"github.com/stanza-go/standalone/module/notifications"
	"github.com/stanza-go/standalone/module/webhooks"
	"github.com/stanza-go/standalone/module/adminroles"
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
	"github.com/stanza-go/standalone/module/usernotifications"
	"github.com/stanza-go/standalone/module/userprofile"
	"github.com/stanza-go/standalone/module/useruploads"
	"github.com/stanza-go/standalone/module/usermgmt"
	"github.com/stanza-go/standalone/module/useractivity"
	"github.com/stanza-go/standalone/module/userapikeys"
	"github.com/stanza-go/standalone/module/userreset"
	"github.com/stanza-go/standalone/module/usersettings"
	"github.com/stanza-go/standalone/seed"
)

// Build metadata — injected at compile time via -ldflags. These remain
// empty strings in development (go run .) and are set to real values in
// production builds (make build / Dockerfile).
var (
	version   string // semantic version tag (e.g. "v0.1.0"), or "dev"
	commit    string // short git commit hash (e.g. "a20cce5")
	buildTime string // UTC build timestamp (e.g. "2026-03-22T10:30:00Z")
)

// signingKey holds the shared JWT signing key used by both admin and
// user auth instances.
type signingKey struct{ key []byte }

// userAuth wraps auth.Auth with user-specific cookie paths (/api
// instead of /api/admin). A distinct type so the DI container can
// provide both admin and user auth instances.
type userAuth struct{ *auth.Auth }

func main() {
	v := version
	if v == "" {
		v = "dev"
	}

	cli := cmd.New("standalone",
		cmd.WithVersion(v),
		cmd.WithDescription("Stanza standalone application server"),
		cmd.WithDefaultCommand("serve"),
	)

	cli.Command("serve", "Start the application server", serveCmd)
	cli.Command("version", "Print version and build information", versionCmd)
	cli.Command("check", "Validate configuration and database connectivity", checkCmd)

	if err := cli.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func serveCmd(_ *cmd.Context) error {
	app := lifecycle.New(
		lifecycle.Provide(provideDataDir),
		lifecycle.Provide(provideConfig),
		lifecycle.Provide(provideLogger),
		lifecycle.Provide(provideDB),
		lifecycle.Provide(provideSigningKey),
		lifecycle.Provide(provideAuth),
		lifecycle.Provide(provideUserAuth),
		lifecycle.Provide(provideEmail),
		lifecycle.Provide(provideNotificationService),
		lifecycle.Provide(provideQueue),
		lifecycle.Provide(provideWebhookDispatcher),
		lifecycle.Provide(provideCron),
		lifecycle.Provide(provideMetrics),
		lifecycle.Provide(provideRouter),
		lifecycle.Provide(provideServer),
		lifecycle.Invoke(registerModules),
	)

	return app.Run()
}

func versionCmd(_ *cmd.Context) error {
	v := version
	if v == "" {
		v = "dev"
	}
	c := commit
	if c == "" {
		c = "unknown"
	}
	bt := buildTime
	if bt == "" {
		bt = "unknown"
	}

	fmt.Printf("standalone %s\n", v)
	fmt.Printf("  commit:     %s\n", c)
	fmt.Printf("  built at:   %s\n", bt)
	fmt.Printf("  go version: %s\n", runtime.Version())
	return nil
}

func checkCmd(_ *cmd.Context) error {
	// 1. Data directory.
	dir, err := datadir.Resolve()
	if err != nil {
		return fmt.Errorf("data directory: %w", err)
	}
	fmt.Printf("data dir:    %s\n", dir.Root)

	// 2. Config.
	cfg, err := config.Load(dir.Config,
		config.WithEnvPrefix("STANZA"),
		config.WithDefaults(map[string]string{
			"server.addr": ":23710",
			"log.level":   "info",
		}),
	)
	if err != nil {
		fmt.Printf("config:      %s (defaults only)\n", dir.Config)
		cfg = config.New(
			config.WithEnvPrefix("STANZA"),
			config.WithDefaults(map[string]string{
				"server.addr": ":23710",
				"log.level":   "info",
			}),
		)
	} else {
		fmt.Printf("config:      %s\n", dir.Config)
	}
	fmt.Printf("server addr: %s\n", cfg.GetStringOr("server.addr", ":23710"))
	fmt.Printf("log level:   %s\n", cfg.GetStringOr("log.level", "info"))

	// 3. Database connectivity.
	db := sqlite.New(dir.DB)
	if err := db.Start(context.Background()); err != nil {
		return fmt.Errorf("database: %w", err)
	}

	if info, err := os.Stat(dir.DB); err == nil {
		fmt.Printf("database:    %s (%s)\n", dir.DB, formatBytes(info.Size()))
	} else {
		fmt.Printf("database:    %s (new)\n", dir.DB)
	}

	// Count applied migrations.
	var migrationCount int
	row := db.QueryRow("SELECT count(*) FROM _migrations")
	if err := row.Scan(&migrationCount); err != nil {
		fmt.Printf("migrations:  unknown (table may not exist yet)\n")
	} else {
		fmt.Printf("migrations:  %d applied\n", migrationCount)
	}

	if err := db.Stop(context.Background()); err != nil {
		return fmt.Errorf("database close: %w", err)
	}

	// 4. Optional services.
	if cfg.GetString("auth.signing_key") != "" {
		fmt.Printf("signing key: configured\n")
	} else {
		fmt.Printf("signing key: not set (random key on each start)\n")
	}

	if cfg.GetString("email.resend_api_key") != "" {
		fmt.Printf("email:       configured\n")
	} else {
		fmt.Printf("email:       not configured (emails disabled)\n")
	}

	fmt.Printf("\nAll checks passed.\n")
	return nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
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
	db := sqlite.New(dir.DB,
		sqlite.WithLogger(logger.With(log.String("pkg", "sqlite"))),
	)

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
			// PRAGMA optimize updates query planner statistics before
			// shutdown. Lightweight — only analyzes tables that need it.
			if err := db.Optimize(); err != nil {
				logger.Warn("pragma optimize failed", log.Err(err))
			}
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

func provideEmail(cfg *config.Config, logger *log.Logger) *email.Client {
	apiKey := cfg.GetString("email.resend_api_key")
	from := cfg.GetStringOr("email.from", "noreply@stanza.dev")

	if apiKey == "" {
		logger.Warn("email.resend_api_key not set — emails will not be sent")
	}

	return email.New(apiKey, email.WithFrom(from))
}

func provideNotificationService(db *sqlite.DB, emailClient *email.Client, logger *log.Logger) *notifications.Service {
	return notifications.NewService(db, emailClient, logger)
}

func provideWebhookDispatcher(db *sqlite.DB, q *queue.Queue, logger *log.Logger) *webhooks.Dispatcher {
	return webhooks.NewDispatcher(db, q, logger)
}

func provideQueue(lc *lifecycle.Lifecycle, db *sqlite.DB, logger *log.Logger) *queue.Queue {
	q := queue.New(db,
		queue.WithLogger(logger),
		queue.WithDefaultTimeout(5*time.Minute),
	)

	lc.Append(lifecycle.Hook{
		OnStart: q.Start,
		OnStop:  q.Stop,
	})

	return q
}

func provideCron(lc *lifecycle.Lifecycle, db *sqlite.DB, q *queue.Queue, dir *datadir.Dir, logger *log.Logger) (*cron.Scheduler, error) {
	s := cron.NewScheduler(
		cron.WithLogger(logger),
		cron.WithDefaultTimeout(10*time.Minute),
		cron.WithOnComplete(func(r cron.CompletedRun) {
			errMsg := ""
			status := "success"
			if r.Err != nil {
				errMsg = r.Err.Error()
				status = "error"
			}
			_, err := db.Insert(sqlite.Insert("cron_runs").
				Set("name", r.Name).
				Set("started_at", sqlite.FormatTime(r.Started)).
				Set("duration_ms", r.Duration.Milliseconds()).
				Set("status", status).
				Set("error", errMsg))
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
		now := sqlite.Now()
		n, err := db.Delete(sqlite.Delete("refresh_tokens").Where("expires_at < ?", now))
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("purged expired refresh tokens", log.Int64("count", n))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-expired-tokens: %w", err)
	}

	// Purge revoked/expired API keys older than 30 days, daily at 3:00 AM.
	if err := s.Add("purge-stale-api-keys", "0 3 * * *", func(ctx context.Context) error {
		cutoff := sqlite.FormatTime(time.Now().Add(-30 * 24 * time.Hour))
		n, err := db.Delete(sqlite.Delete("api_keys").
			WhereOr(
				sqlite.Cond("revoked_at IS NOT NULL AND revoked_at < ?", cutoff),
				sqlite.Cond("expires_at IS NOT NULL AND expires_at < ?", cutoff),
			))
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("purged stale API keys", log.Int64("count", n))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-stale-api-keys: %w", err)
	}

	// Purge cron run history older than 7 days, daily at 3:30 AM.
	if err := s.Add("purge-old-cron-runs", "30 3 * * *", func(ctx context.Context) error {
		cutoff := sqlite.FormatTime(time.Now().Add(-7 * 24 * time.Hour))
		n, err := db.Delete(sqlite.Delete("cron_runs").Where("started_at < ?", cutoff))
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("purged old cron runs", log.Int64("count", n))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-old-cron-runs: %w", err)
	}

	// Purge audit log entries older than 90 days, daily at 4:00 AM.
	if err := s.Add("purge-old-audit-log", "0 4 * * *", func(ctx context.Context) error {
		cutoff := sqlite.FormatTime(time.Now().Add(-90 * 24 * time.Hour))
		n, err := db.Delete(sqlite.Delete("audit_log").Where("created_at < ?", cutoff))
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("purged old audit log entries", log.Int64("count", n))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-old-audit-log: %w", err)
	}

	// Purge used/expired password reset tokens older than 24h, daily at 4:30 AM.
	if err := s.Add("purge-old-reset-tokens", "30 4 * * *", func(ctx context.Context) error {
		cutoff := sqlite.FormatTime(time.Now().Add(-24 * time.Hour))
		n, err := db.Delete(sqlite.Delete("password_reset_tokens").
			WhereOr(
				sqlite.Cond("used_at IS NOT NULL"),
				sqlite.Cond("expires_at < ?", cutoff),
			))
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("purged old password reset tokens", log.Int64("count", n))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-old-reset-tokens: %w", err)
	}

	// Purge read notifications older than 30 days, daily at 5:00 AM.
	if err := s.Add("purge-old-notifications", "0 5 * * *", func(ctx context.Context) error {
		cutoff := sqlite.FormatTime(time.Now().Add(-30 * 24 * time.Hour))
		n, err := db.Delete(sqlite.Delete("notifications").
			WhereNotNull("read_at").
			Where("created_at < ?", cutoff))
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("purged old read notifications", log.Int64("count", n))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-old-notifications: %w", err)
	}

	// Purge webhook deliveries older than 30 days, daily at 5:30 AM.
	if err := s.Add("purge-old-webhook-deliveries", "30 5 * * *", func(ctx context.Context) error {
		cutoff := sqlite.FormatTime(time.Now().Add(-30 * 24 * time.Hour))
		n, err := db.Delete(sqlite.Delete("webhook_deliveries").Where("created_at < ?", cutoff))
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("purged old webhook deliveries", log.Int64("count", n))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-old-webhook-deliveries: %w", err)
	}

	// Purge soft-deleted uploads older than 30 days (DB records + files on disk),
	// daily at 6:00 AM. Queries storage paths first to remove files, then
	// hard-deletes the records.
	if err := s.Add("purge-deleted-uploads", "0 6 * * *", func(ctx context.Context) error {
		cutoff := sqlite.FormatTime(time.Now().Add(-30 * 24 * time.Hour))

		// Get storage paths of soft-deleted uploads to remove files from disk.
		sql, args := sqlite.Select("storage_path").From("uploads").
			WhereNotNull("deleted_at").
			Where("deleted_at < ?", cutoff).
			Build()
		paths, err := sqlite.QueryAll(db, sql, args, func(rows *sqlite.Rows) (string, error) {
			var p string
			err := rows.Scan(&p)
			return p, err
		})
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			return nil
		}

		// Remove upload directories from disk. Each storage_path is
		// "YYYY/MM/DD/uuid/filename" — removing the parent (uuid dir)
		// deletes both the file and its thumbnail.
		var filesRemoved int
		for _, p := range paths {
			if p == "" {
				continue
			}
			uuidDir := filepath.Join(dir.Uploads, filepath.Dir(p))
			if err := os.RemoveAll(uuidDir); err == nil {
				filesRemoved++
			}
		}

		// Hard-delete DB records.
		n, err := db.Delete(sqlite.Delete("uploads").
			WhereNotNull("deleted_at").
			Where("deleted_at < ?", cutoff))
		if err != nil {
			return err
		}

		logger.Info("purged deleted uploads",
			log.Int64("records", n),
			log.Int("directories", filesRemoved),
		)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-deleted-uploads: %w", err)
	}

	// Automated daily backup at 2:00 AM — uses VACUUM INTO for a consistent,
	// compacted copy that includes all WAL data. 30-minute timeout because
	// VACUUM INTO can be slow on large databases.
	if err := s.Add("daily-backup", "0 2 * * *", func(ctx context.Context) error {
		ts := time.Now().UTC().Format("20060102T150405Z")
		backupName := fmt.Sprintf("database.sqlite.%s.bak", ts)
		backupPath := filepath.Join(dir.Backups, backupName)

		if err := db.Backup(backupPath); err != nil {
			return fmt.Errorf("backup database: %w", err)
		}

		info, err := os.Stat(backupPath)
		if err != nil {
			return fmt.Errorf("stat backup: %w", err)
		}

		logger.Info("daily backup completed",
			log.String("file", backupName),
			log.Int64("size_bytes", info.Size()),
		)
		return nil
	}, cron.Timeout(30*time.Minute)); err != nil {
		return nil, fmt.Errorf("cron add daily-backup: %w", err)
	}

	// Purge backups older than 7 days, daily at 2:30 AM.
	if err := s.Add("purge-old-backups", "30 2 * * *", func(ctx context.Context) error {
		cutoff := time.Now().Add(-7 * 24 * time.Hour)
		entries, err := os.ReadDir(dir.Backups)
		if err != nil {
			return fmt.Errorf("read backups dir: %w", err)
		}

		var removed int
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				if err := os.Remove(filepath.Join(dir.Backups, e.Name())); err == nil {
					removed++
				}
			}
		}
		if removed > 0 {
			logger.Info("purged old backups", log.Int("count", removed))
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("cron add purge-old-backups: %w", err)
	}

	lc.Append(lifecycle.Hook{
		OnStart: s.Start,
		OnStop:  s.Stop,
	})

	return s, nil
}

func provideMetrics() *http.Metrics {
	return http.NewMetrics()
}

func provideRouter(logger *log.Logger, cfg *config.Config, m *http.Metrics) *http.Router {
	router := http.NewRouter()

	router.Use(http.RequestID(http.RequestIDConfig{}))
	router.Use(m.Middleware())
	router.Use(http.RequestLogger(logger))
	router.Use(http.Compress(http.CompressConfig{}))
	router.Use(http.ETag(http.ETagConfig{}))
	secCfg := http.SecureHeadersConfig{}
	// Enable HSTS when running behind a TLS-terminating proxy (Railway,
	// Cloud Run, etc.). The PORT env var is the standard signal that the
	// app is running in a container platform — these always serve HTTPS.
	if os.Getenv("PORT") != "" {
		secCfg.HSTSMaxAge = 31536000 // 1 year
	}
	router.Use(http.SecureHeaders(secCfg))
	router.Use(http.MaxBody(2 << 20)) // 2 MB — multipart uploads exempt

	// CORS: allow cross-origin requests from admin and UI dev servers.
	// Configure via STANZA_CORS_ORIGINS (comma-separated) or cors.origins
	// in config.yaml. Defaults to the admin and UI Vite dev servers.
	originsStr := cfg.GetStringOr("cors.origins", "http://localhost:23706,http://localhost:23700")
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
	// Railway, Cloud Run, etc. set PORT — always prefer it when present.
	if port := os.Getenv("PORT"); port != "" {
		addr = "0.0.0.0:" + port
	}
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

func registerModules(router *http.Router, db *sqlite.DB, a *auth.Auth, ua *userAuth, q *queue.Queue, s *cron.Scheduler, m *http.Metrics, dir *datadir.Dir, emailClient *email.Client, notifSvc *notifications.Service, whDispatcher *webhooks.Dispatcher, logger *log.Logger) {
	api := router.Group("/api")

	// Public routes.
	health.Register(api, db, health.BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
	})
	api.HandleFunc("GET /metrics", http.PrometheusHandler(
		collectPrometheus(db, m, q, s, whDispatcher, a, emailClient),
	))

	// Auth routes — rate limited to prevent brute force attacks.
	// 20 requests per minute per IP covers legitimate use (including
	// status polling from multiple tabs) while stopping automated attacks.
	authRL := api.Group("")
	authRL.Use(http.RateLimit(http.RateLimitConfig{
		Limit:   20,
		Window:  time.Minute,
		Message: "too many requests, please try again later",
	}))
	adminauth.Register(authRL, a, db)
	userauth.Register(authRL, ua.Auth, db, whDispatcher)
	userreset.Register(authRL, db, emailClient)

	// Protected admin routes — require valid JWT + admin scope.
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))

	// Dashboard and profile: base admin scope only.
	dashboard.Register(admin, db, q, s, m, whDispatcher, a, emailClient)
	adminprofile.Register(admin, db)

	// Scoped admin sub-groups — each module gets its specific scope.
	withUsers := admin.Group("")
	withUsers.Use(auth.RequireScope("admin:users"))
	adminusers.Register(withUsers, db, whDispatcher)
	adminsessions.Register(withUsers, db, whDispatcher)
	usermgmt.Register(withUsers, a, db, whDispatcher)
	apikeys.Register(withUsers, db)

	withSettings := admin.Group("")
	withSettings.Use(auth.RequireScope("admin:settings"))
	adminsettings.Register(withSettings, db, whDispatcher)

	withJobs := admin.Group("")
	withJobs.Use(auth.RequireScope("admin:jobs"))
	admincron.Register(withJobs, s, db)
	adminqueue.Register(withJobs, q, db)

	withLogs := admin.Group("")
	withLogs.Use(auth.RequireScope("admin:logs"))
	adminlogs.Register(withLogs, dir.Logs)

	withDB := admin.Group("")
	withDB.Use(auth.RequireScope("admin:database"))
	admindb.Register(withDB, db, dir.Backups)

	withAudit := admin.Group("")
	withAudit.Use(auth.RequireScope("admin:audit"))
	adminaudit.Register(withAudit, db)

	// Roles module handles its own scope check internally.
	adminroles.Register(admin, db, whDispatcher)

	withUploads := admin.Group("")
	withUploads.Use(auth.RequireScope("admin:uploads"))
	adminuploads.Register(withUploads, db, dir.Uploads)

	withNotifications := admin.Group("")
	withNotifications.Use(auth.RequireScope("admin:notifications"))
	adminnotifications.Register(withNotifications, db, notifSvc)

	withWebhooks := admin.Group("")
	withWebhooks.Use(auth.RequireScope("admin:webhooks"))
	adminwebhooks.Register(withWebhooks, db, whDispatcher)

	// API key validator — shared between user routes and v1 routes.
	kv := apikeys.NewValidator(db)

	// Protected user routes — require valid JWT or API key + user scope.
	// RequireAuthOrAPIKey tries JWT cookie first, then falls back to
	// Bearer token API key auth, enabling programmatic access.
	user := api.Group("/user")
	user.Use(ua.RequireAuthOrAPIKey(kv))
	user.Use(auth.RequireScope("user"))

	userprofile.Register(user, db, whDispatcher)
	usernotifications.Register(user, db)
	useruploads.Register(user, db, dir.Uploads)
	userapikeys.Register(user, db)
	usersettings.Register(user, db)
	useractivity.Register(user, db)

	// API key authenticated routes — for programmatic access.
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

// collectPrometheus returns a collector function that gathers metrics
// from all framework packages for Prometheus exposition. Called on each
// scrape of GET /api/metrics.
func collectPrometheus(db *sqlite.DB, m *http.Metrics, q *queue.Queue, s *cron.Scheduler, wh *webhooks.Dispatcher, a *auth.Auth, ec *email.Client) func() []http.PrometheusMetric {
	return func() []http.PrometheusMetric {
		var out []http.PrometheusMetric

		// SQLite.
		ds := db.Stats()
		out = append(out,
			http.PrometheusMetric{Name: "stanza_sqlite_reads_total", Help: "Total read queries executed", Type: "counter", Value: float64(ds.TotalReads)},
			http.PrometheusMetric{Name: "stanza_sqlite_writes_total", Help: "Total write queries executed", Type: "counter", Value: float64(ds.TotalWrites)},
			http.PrometheusMetric{Name: "stanza_sqlite_pool_waits_total", Help: "Total read pool wait events", Type: "counter", Value: float64(ds.PoolWaits)},
			http.PrometheusMetric{Name: "stanza_sqlite_read_pool_size", Help: "Read pool total connections", Type: "gauge", Value: float64(ds.ReadPoolSize)},
			http.PrometheusMetric{Name: "stanza_sqlite_read_pool_in_use", Help: "Read pool connections currently in use", Type: "gauge", Value: float64(ds.ReadPoolInUse)},
			http.PrometheusMetric{Name: "stanza_sqlite_file_size_bytes", Help: "Main database file size in bytes", Type: "gauge", Value: float64(ds.FileSize)},
			http.PrometheusMetric{Name: "stanza_sqlite_wal_size_bytes", Help: "WAL file size in bytes", Type: "gauge", Value: float64(ds.WALSize)},
		)

		// HTTP.
		hs := m.Stats()
		out = append(out,
			http.PrometheusMetric{Name: "stanza_http_requests_total", Help: "Total HTTP requests processed", Type: "counter", Value: float64(hs.TotalRequests)},
			http.PrometheusMetric{Name: "stanza_http_requests_active", Help: "HTTP requests currently being processed", Type: "gauge", Value: float64(hs.ActiveRequests)},
			http.PrometheusMetric{Name: "stanza_http_responses_2xx_total", Help: "Total 2xx responses", Type: "counter", Value: float64(hs.Status2xx)},
			http.PrometheusMetric{Name: "stanza_http_responses_3xx_total", Help: "Total 3xx responses", Type: "counter", Value: float64(hs.Status3xx)},
			http.PrometheusMetric{Name: "stanza_http_responses_4xx_total", Help: "Total 4xx responses", Type: "counter", Value: float64(hs.Status4xx)},
			http.PrometheusMetric{Name: "stanza_http_responses_5xx_total", Help: "Total 5xx responses", Type: "counter", Value: float64(hs.Status5xx)},
			http.PrometheusMetric{Name: "stanza_http_response_bytes_total", Help: "Total response bytes written", Type: "counter", Value: float64(hs.BytesWritten)},
			http.PrometheusMetric{Name: "stanza_http_request_duration_avg_ms", Help: "Average request duration in milliseconds", Type: "gauge", Value: hs.AvgDurationMs},
		)

		// Queue.
		qs, err := q.Stats()
		if err == nil {
			out = append(out,
				http.PrometheusMetric{Name: "stanza_queue_pending", Help: "Jobs waiting to be processed", Type: "gauge", Value: float64(qs.Pending)},
				http.PrometheusMetric{Name: "stanza_queue_running", Help: "Jobs currently being processed", Type: "gauge", Value: float64(qs.Running)},
				http.PrometheusMetric{Name: "stanza_queue_completed_total", Help: "Total jobs completed successfully", Type: "counter", Value: float64(qs.Completed)},
				http.PrometheusMetric{Name: "stanza_queue_failed_total", Help: "Total jobs that failed", Type: "counter", Value: float64(qs.Failed)},
				http.PrometheusMetric{Name: "stanza_queue_dead_total", Help: "Total jobs that exceeded max retries", Type: "counter", Value: float64(qs.Dead)},
				http.PrometheusMetric{Name: "stanza_queue_cancelled_total", Help: "Total jobs cancelled", Type: "counter", Value: float64(qs.Cancelled)},
			)
		}

		// Cron.
		cs := s.Stats()
		out = append(out,
			http.PrometheusMetric{Name: "stanza_cron_jobs_registered", Help: "Number of registered cron jobs", Type: "gauge", Value: float64(cs.Jobs)},
			http.PrometheusMetric{Name: "stanza_cron_completed_total", Help: "Total cron runs completed", Type: "counter", Value: float64(cs.Completed)},
			http.PrometheusMetric{Name: "stanza_cron_failed_total", Help: "Total cron runs that errored", Type: "counter", Value: float64(cs.Failed)},
			http.PrometheusMetric{Name: "stanza_cron_skipped_total", Help: "Total cron runs skipped (previous still running)", Type: "counter", Value: float64(cs.Skipped)},
		)

		// Webhook.
		ws := wh.Stats()
		out = append(out,
			http.PrometheusMetric{Name: "stanza_webhook_sends_total", Help: "Total webhook delivery attempts", Type: "counter", Value: float64(ws.Sends)},
			http.PrometheusMetric{Name: "stanza_webhook_successes_total", Help: "Total successful webhook deliveries", Type: "counter", Value: float64(ws.Successes)},
			http.PrometheusMetric{Name: "stanza_webhook_failures_total", Help: "Total failed webhook deliveries", Type: "counter", Value: float64(ws.Failures)},
			http.PrometheusMetric{Name: "stanza_webhook_retries_total", Help: "Total webhook delivery retries", Type: "counter", Value: float64(ws.Retries)},
		)

		// Auth.
		as := a.Stats()
		out = append(out,
			http.PrometheusMetric{Name: "stanza_auth_tokens_issued_total", Help: "Total JWT tokens issued", Type: "counter", Value: float64(as.Issued)},
			http.PrometheusMetric{Name: "stanza_auth_tokens_accepted_total", Help: "Total JWT tokens validated successfully", Type: "counter", Value: float64(as.Accepted)},
			http.PrometheusMetric{Name: "stanza_auth_tokens_rejected_total", Help: "Total JWT tokens rejected", Type: "counter", Value: float64(as.Rejected)},
		)

		// Email.
		es := ec.Stats()
		out = append(out,
			http.PrometheusMetric{Name: "stanza_email_sent_total", Help: "Total emails sent successfully", Type: "counter", Value: float64(es.Sent)},
			http.PrometheusMetric{Name: "stanza_email_errors_total", Help: "Total email send failures", Type: "counter", Value: float64(es.Errors)},
		)

		// Go runtime.
		out = append(out, http.RuntimeMetrics()...)

		return out
	}
}
