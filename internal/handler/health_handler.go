package handler

import (
	"context"
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HealthHandler reports service liveness, including database connectivity.
type HealthHandler struct {
	db *sql.DB
}

// NewHealthHandler builds a health handler bound to the DB connection pool.
func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Check handles GET /health. It pings the database and returns a JSON body of
// the form:
//
//	{ "status": "ok", "db": "connected", "timestamp": "2026-06-13T..." }
//
// If the DB ping fails, status is "degraded", db is "disconnected", and the
// response code is 503 so orchestrators can detect the unhealthy state.
func (h *HealthHandler) Check(c *fiber.Ctx) error {
	dbStatus := "connected"
	status := "ok"
	code := fiber.StatusOK

	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	if h.db == nil || h.db.PingContext(ctx) != nil {
		dbStatus = "disconnected"
		status = "degraded"
		code = fiber.StatusServiceUnavailable
	}

	return c.Status(code).JSON(fiber.Map{
		"status":    status,
		"db":        dbStatus,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
