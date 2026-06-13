package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Vamsi-Kathi/ainyx-user-api/internal/models"
)

// RequestIDHeader is the HTTP header carrying the per-request correlation id.
const RequestIDHeader = "X-Request-ID"

// requestIDKey is the key under which the request id is stored in Fiber locals.
const requestIDKey = "request_id"

// RequestID injects a UUID v4 into every request/response. If the client
// supplies an X-Request-ID it is honored; otherwise a new one is generated.
// The id is stored in c.Locals so downstream handlers/middleware can read it.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		rid := c.Get(RequestIDHeader)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Locals(requestIDKey, rid)
		c.Set(RequestIDHeader, rid)
		return c.Next()
	}
}

// GetRequestID retrieves the request id stored by the RequestID middleware.
func GetRequestID(c *fiber.Ctx) string {
	if rid, ok := c.Locals(requestIDKey).(string); ok {
		return rid
	}
	return ""
}

// ZapLogger logs one structured line per request using Uber Zap, including
// method, path, status, duration and the request id.
func ZapLogger(log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request; capture any error so we can still log the line.
		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		fields := []zap.Field{
			zap.String("method", c.Method()),
			zap.String("path", c.OriginalURL()),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("request_id", GetRequestID(c)),
			zap.String("ip", c.IP()),
		}

		switch {
		case status >= 500:
			log.Error("request completed", fields...)
		case status >= 400:
			log.Warn("request completed", fields...)
		default:
			log.Info("request completed", fields...)
		}

		return err
	}
}

// CORS allows all origins so the single-file frontend can call the API.
func CORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:  "*",
		AllowMethods:  "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:  "Origin,Content-Type,Accept,X-Request-ID",
		ExposeHeaders: RequestIDHeader,
	})
}

// ErrorHandler is Fiber's centralized error handler. It converts any error
// returned from a handler/middleware into the consistent JSON error envelope
// and logs server-side failures with a stack trace.
func ErrorHandler(log *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		msg := "internal server error"

		// Honor Fiber's typed errors (e.g. fiber.NewError(404, "...")).
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
			msg = e.Message
		}

		rid := GetRequestID(c)

		if code >= 500 {
			log.Error("request failed",
				zap.String("request_id", rid),
				zap.String("method", c.Method()),
				zap.String("path", c.OriginalURL()),
				zap.Int("status", code),
				zap.Error(err),
			)
		} else {
			log.Warn("request rejected",
				zap.String("request_id", rid),
				zap.String("method", c.Method()),
				zap.String("path", c.OriginalURL()),
				zap.Int("status", code),
				zap.String("error", msg),
			)
		}

		return c.Status(code).JSON(models.ErrorResponse{
			Error:     msg,
			RequestID: rid,
		})
	}
}
