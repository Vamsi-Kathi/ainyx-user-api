package routes

import (
	"github.com/gofiber/fiber/v2"

	"github.com/Vamsi-Kathi/ainyx-user-api/internal/handler"
)

// Register mounts all application routes onto the Fiber app.
func Register(app *fiber.App, h *handler.UserHandler, health *handler.HealthHandler) {
	// Health check for container orchestration / load balancers; reports DB
	// connectivity and a timestamp.
	app.Get("/health", health.Check)

	users := app.Group("/users")
	users.Post("/", h.Create)
	users.Get("/", h.List)
	users.Get("/:id", h.Get)
	users.Put("/:id", h.Update)
	users.Delete("/:id", h.Delete)
}
