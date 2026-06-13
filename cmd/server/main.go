package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/Vamsi-Kathi/ainyx-user-api/config"
	db "github.com/Vamsi-Kathi/ainyx-user-api/db/sqlc"
	"github.com/Vamsi-Kathi/ainyx-user-api/internal/handler"
	"github.com/Vamsi-Kathi/ainyx-user-api/internal/logger"
	"github.com/Vamsi-Kathi/ainyx-user-api/internal/middleware"
	"github.com/Vamsi-Kathi/ainyx-user-api/internal/routes"
	"github.com/Vamsi-Kathi/ainyx-user-api/internal/service"
)

func main() {
	cfg := config.Load()

	log, err := logger.New(cfg.LogLevel, cfg.IsProduction())
	if err != nil {
		panic("failed to init logger: " + err.Error())
	}
	defer func() { _ = log.Sync() }()

	// Connect to MySQL with retries (the DB container may still be starting).
	sqlDB, err := connectDB(cfg, log)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer func() { _ = sqlDB.Close() }()

	// Build the dependency chain: queries -> service -> handler.
	queries := db.New(sqlDB)
	userSvc := service.NewUserService(queries, log)
	userHandler := handler.NewUserHandler(userSvc, log)
	healthHandler := handler.NewHealthHandler(sqlDB)

	app := fiber.New(fiber.Config{
		AppName:               "ainyx-user-api",
		ErrorHandler:          middleware.ErrorHandler(log),
		DisableStartupMessage: true,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
	})

	// Global middleware order matters: request id first so it is available to
	// the logger and error handler.
	app.Use(middleware.RequestID())
	app.Use(middleware.CORS())
	app.Use(middleware.ZapLogger(log))

	routes.Register(app, userHandler, healthHandler)

	// Run the server in a goroutine so we can listen for shutdown signals.
	go func() {
		addr := ":" + cfg.AppPort
		log.Info("server starting", zap.String("addr", addr), zap.String("env", cfg.AppEnv))
		if err := app.Listen(addr); err != nil && !errors.Is(err, fiber.ErrServiceUnavailable) {
			log.Fatal("server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("server stopped")
}

// connectDB opens a MySQL connection pool and verifies it with a ping,
// retrying for up to ~30s so the app survives a slow-starting DB container.
func connectDB(cfg *config.Config, log *zap.Logger) (*sql.DB, error) {
	sqlDB, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	const maxAttempts = 15
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = sqlDB.PingContext(ctx)
		cancel()
		if err == nil {
			log.Info("connected to database", zap.String("host", cfg.DBHost), zap.String("db", cfg.DBName))
			return sqlDB, nil
		}
		log.Warn("database not ready, retrying...",
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", maxAttempts),
			zap.Error(err),
		)
		time.Sleep(2 * time.Second)
	}
	return nil, err
}
