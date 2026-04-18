package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edge-platform/server/internal/api/middleware"
	"github.com/edge-platform/server/internal/api/routes"
	"github.com/edge-platform/server/internal/config"
	"github.com/edge-platform/server/internal/domain/models"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/service"
	"github.com/edge-platform/server/internal/websocket"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := config.Load(""); err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	cfg := config.Get()

	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	db := initDB(cfg)
	redisClient, err := pkgRedis.InitializeRedis(cfg)
	if err != nil {
		slog.Error("failed to initialize Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	if err := models.InitializeDatabase(db); err != nil {
		slog.Error("failed to run database migration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	heartbeatWorker := service.NewHeartbeatWorker(db)
	if err := heartbeatWorker.Start(ctx); err != nil {
		slog.Error("failed to start heartbeat worker", "error", err)
		os.Exit(1)
	}

	auditService := service.NewAuditService(db, redisClient, service.AuditConfig{
		QueueSize: cfg.Audit.QueueSize,
		BatchSize: cfg.Audit.BatchSize,
	})
	if cfg.Audit.Enabled {
		auditService.Start(ctx)
	}

	r := gin.Default()

	if cfg.Audit.Enabled {
		r.Use(middleware.AuditMiddleware(&middleware.AuditMiddlewareOptions{
			AuditService: auditService,
			SkipPaths:    []string{"/health", "/metrics"},
		}))
	}

	hub := websocket.NewHub()
	gw := websocket.NewGateway(redisClient.Raw(), hub)
	gw.SetMessageHandler(nil)

	routes.SetupAuthRoutes(r, db, redisClient)
	routes.SetupDeviceRoutes(r, db, redisClient)
	routes.SetupGroupRoutes(r, db)
	routes.SetupTerminalRoutes(r, db, redisClient, gw)
	routes.SetupFileRoutes(r, db, redisClient, gw)
	routes.SetupAuditRoutes(r, db, auditService)

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")
	cancel()
	heartbeatWorker.Stop()
	if cfg.Audit.Enabled {
		auditService.Stop()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server exited gracefully")
}

func initDB(cfg *config.Config) *gorm.DB {
	logLevel := logger.Silent
	if cfg.App.Debug {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DB.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("failed to get sql.DB", "error", err)
		os.Exit(1)
	}

	sqlDB.SetMaxOpenConns(cfg.DB.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DB.MaxIdleConns)

	return db
}
