package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"

	db "github.com/Vamsi-Kathi/ainyx-user-api/db/sqlc"
	"github.com/Vamsi-Kathi/ainyx-user-api/internal/models"
)

// Sentinel errors returned by the service layer. Handlers map these to HTTP
// status codes so the service stays transport-agnostic. The messages are the
// exact, user-facing strings surfaced in the API error envelope.
var (
	ErrNotFound   = errors.New("user not found")
	ErrInvalidDOB = errors.New("use format YYYY-MM-DD")
	ErrFutureDOB  = errors.New("date of birth cannot be in the future")
	ErrDOBTooOld  = errors.New("date of birth seems invalid")
	ErrTooYoung   = errors.New("person must be at least 1 year old")
)

// minDOB is the earliest date of birth we consider plausible. Anything before
// this is treated as a data-entry error rather than a real birth date.
var minDOB = time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC)

// UserService contains the business logic for the user resource.
type UserService struct {
	q   db.Querier
	log *zap.Logger
}

// NewUserService wires a service with its data store and logger.
func NewUserService(q db.Querier, log *zap.Logger) *UserService {
	return &UserService{q: q, log: log}
}

// now is overridable in tests; defaults to the real clock.
var now = time.Now

// parseDOB parses a YYYY-MM-DD string and enforces all date-of-birth rules,
// returning a specific sentinel error for each failure mode:
//   - bad format         -> ErrInvalidDOB  ("use format YYYY-MM-DD")
//   - in the future      -> ErrFutureDOB   ("date of birth cannot be in the future")
//   - before 1900-01-01  -> ErrDOBTooOld   ("date of birth seems invalid")
//   - under 1 year old   -> ErrTooYoung    ("person must be at least 1 year old")
//
// time.Parse with the strict YYYY-MM-DD layout rejects any other format,
// including datetimes, slashes, and out-of-range month/day values.
func parseDOB(s string) (time.Time, error) {
	dob, err := time.Parse(models.DateLayout, s)
	if err != nil {
		return time.Time{}, ErrInvalidDOB
	}
	dobUTC := dob.UTC()

	current := now().UTC()
	if !dobUTC.Before(current) {
		return time.Time{}, ErrFutureDOB
	}

	if dobUTC.Before(minDOB) {
		return time.Time{}, ErrDOBTooOld
	}

	if CalculateAge(dob, current) < 1 {
		return time.Time{}, ErrTooYoung
	}

	return dob, nil
}

// Create validates the request and persists a new user.
func (s *UserService) Create(ctx context.Context, req models.CreateUserRequest) (*models.UserResponse, error) {
	dob, err := parseDOB(req.Dob)
	if err != nil {
		return nil, err
	}

	res, err := s.q.CreateUser(ctx, db.CreateUserParams{
		Name: req.Name,
		Dob:  dob,
	})
	if err != nil {
		s.log.Error("db create user failed", zap.Error(err), zap.String("name", req.Name))
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		s.log.Error("db lastInsertId failed", zap.Error(err))
		return nil, err
	}

	s.log.Info("db operation", zap.String("action", "create_user"), zap.Int64("user_id", id))

	return &models.UserResponse{
		ID:   uint64(id),
		Name: req.Name,
		Dob:  req.Dob,
	}, nil
}

// Get fetches a single user including the calculated age.
func (s *UserService) Get(ctx context.Context, id uint64) (*models.UserWithAgeResponse, error) {
	u, err := s.q.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		s.log.Error("db get user failed", zap.Error(err), zap.Uint64("user_id", id))
		return nil, err
	}

	s.log.Info("db operation", zap.String("action", "get_user"), zap.Uint64("user_id", id))

	return toUserWithAge(u), nil
}

// List returns a page of users (with ages) plus pagination metadata.
func (s *UserService) List(ctx context.Context, page, limit int) (*models.ListUsersResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	users, err := s.q.ListUsers(ctx, db.ListUsersParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		s.log.Error("db list users failed", zap.Error(err))
		return nil, err
	}

	total, err := s.q.CountUsers(ctx)
	if err != nil {
		s.log.Error("db count users failed", zap.Error(err))
		return nil, err
	}

	data := make([]models.UserWithAgeResponse, 0, len(users))
	for _, u := range users {
		data = append(data, *toUserWithAge(u))
	}

	s.log.Info("db operation",
		zap.String("action", "list_users"),
		zap.Int("page", page),
		zap.Int("limit", limit),
		zap.Int64("total", total),
	)

	return &models.ListUsersResponse{
		Data: data,
		Pagination: models.Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	}, nil
}

// Update validates and persists changes to an existing user. It relies on the
// UPDATE's affected-row count to detect a missing user, which is race-free (no
// check-then-act window) and saves a round-trip versus a separate existence
// query.
func (s *UserService) Update(ctx context.Context, id uint64, req models.UpdateUserRequest) (*models.UserResponse, error) {
	dob, err := parseDOB(req.Dob)
	if err != nil {
		return nil, err
	}

	res, err := s.q.UpdateUser(ctx, db.UpdateUserParams{
		Name: req.Name,
		Dob:  dob,
		ID:   id,
	})
	if err != nil {
		s.log.Error("db update user failed", zap.Error(err), zap.Uint64("user_id", id))
		return nil, err
	}
	if err := requireRowAffected(res); err != nil {
		return nil, err
	}

	s.log.Info("db operation", zap.String("action", "update_user"), zap.Uint64("user_id", id))

	return &models.UserResponse{
		ID:   id,
		Name: req.Name,
		Dob:  req.Dob,
	}, nil
}

// Delete removes a user, returning ErrNotFound if no row matched. Like Update,
// it uses the affected-row count rather than a check-then-delete sequence.
func (s *UserService) Delete(ctx context.Context, id uint64) error {
	res, err := s.q.DeleteUser(ctx, id)
	if err != nil {
		s.log.Error("db delete user failed", zap.Error(err), zap.Uint64("user_id", id))
		return err
	}
	if err := requireRowAffected(res); err != nil {
		return err
	}

	s.log.Info("db operation", zap.String("action", "delete_user"), zap.Uint64("user_id", id))
	return nil
}

// requireRowAffected maps a write that touched no rows to ErrNotFound. A driver
// that cannot report RowsAffected is treated as an internal error rather than a
// silent success.
func requireRowAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// toUserWithAge maps a db.User row to the API response including age.
func toUserWithAge(u db.User) *models.UserWithAgeResponse {
	return &models.UserWithAgeResponse{
		ID:   u.ID,
		Name: u.Name,
		Dob:  u.Dob.Format(models.DateLayout),
		Age:  CalculateAge(u.Dob, now()),
	}
}
