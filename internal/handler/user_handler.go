package handler

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/Vamsi-Kathi/ainyx-user-api/internal/models"
	"github.com/Vamsi-Kathi/ainyx-user-api/internal/service"
)

// UserHandler exposes HTTP handlers for the user resource.
type UserHandler struct {
	svc      *service.UserService
	validate *validator.Validate
	log      *zap.Logger
}

// NewUserHandler builds a handler with a configured validator instance.
func NewUserHandler(svc *service.UserService, log *zap.Logger) *UserHandler {
	v := validator.New()
	// Custom validator: ensures dob is exactly YYYY-MM-DD. Past/min-age checks
	// happen in the service layer where the clock is injectable.
	_ = v.RegisterValidation("dateonly", func(fl validator.FieldLevel) bool {
		_, err := time.Parse(models.DateLayout, fl.Field().String())
		return err == nil
	})

	return &UserHandler{svc: svc, validate: v, log: log}
}

// Create handles POST /users.
func (h *UserHandler) Create(c *fiber.Ctx) error {
	var req models.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	// Trim before validating so the min/max length rules apply to the real
	// content and whitespace-only names are rejected.
	req.Name = strings.TrimSpace(req.Name)
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, validationMessage(err))
	}

	resp, err := h.svc.Create(c.Context(), req)
	if err != nil {
		return mapServiceError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// Get handles GET /users/:id.
func (h *UserHandler) Get(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	resp, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return mapServiceError(err)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// List handles GET /users?page=&limit=.
func (h *UserHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	resp, err := h.svc.List(c.Context(), page, limit)
	if err != nil {
		return mapServiceError(err)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// Update handles PUT /users/:id.
func (h *UserHandler) Update(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	var req models.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, validationMessage(err))
	}

	resp, err := h.svc.Update(c.Context(), id, req)
	if err != nil {
		return mapServiceError(err)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// Delete handles DELETE /users/:id. Returns 204 No Content on success.
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return mapServiceError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// parseID extracts and validates the :id path parameter.
func parseID(c *fiber.Ctx) (uint64, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid user id")
	}
	return id, nil
}

// mapServiceError converts service sentinel errors into HTTP errors handled by
// the centralized error middleware.
func mapServiceError(err error) error {
	switch {
	case errors.Is(err, service.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInvalidDOB),
		errors.Is(err, service.ErrFutureDOB),
		errors.Is(err, service.ErrDOBTooOld),
		errors.Is(err, service.ErrTooYoung):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	default:
		// Unknown error -> 500; the error handler logs it with a stack trace.
		return err
	}
}

// validationMessage turns validator errors into a concise, user-friendly string.
func validationMessage(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) && len(ve) > 0 {
		fe := ve[0]
		field := fe.Field()
		switch fe.Tag() {
		case "required":
			return field + " is required"
		case "min":
			return field + " must be at least " + fe.Param() + " characters"
		case "max":
			return field + " must be at most " + fe.Param() + " characters"
		case "dateonly":
			return "use format YYYY-MM-DD"
		default:
			return "invalid value for " + field
		}
	}
	return "validation failed"
}
