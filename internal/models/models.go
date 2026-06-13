package models

// DateLayout is the canonical date format used across the API for the dob field.
const DateLayout = "2006-01-02"

// CreateUserRequest is the payload for POST /users.
//
// Validation rules:
//   - name: required, 2..100 chars
//   - dob:  required, YYYY-MM-DD, in the past, person at least 1 year old
//
// The dob validations beyond format ("past", "min age 1") are enforced in the
// service layer where time math is available; the custom "dateonly" validator
// guarantees the YYYY-MM-DD shape here.
type CreateUserRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
	Dob  string `json:"dob" validate:"required,dateonly"`
}

// UpdateUserRequest is the payload for PUT /users/:id. Same rules as create.
type UpdateUserRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
	Dob  string `json:"dob" validate:"required,dateonly"`
}

// UserResponse is returned by create/update endpoints (no age).
type UserResponse struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Dob  string `json:"dob"`
}

// UserWithAgeResponse is returned by get/list endpoints (includes age).
type UserWithAgeResponse struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Dob  string `json:"dob"`
	Age  int    `json:"age"`
}

// Pagination describes the paging metadata returned by GET /users.
type Pagination struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

// ListUsersResponse is the envelope for GET /users.
type ListUsersResponse struct {
	Data       []UserWithAgeResponse `json:"data"`
	Pagination Pagination            `json:"pagination"`
}

// ErrorResponse is the consistent error envelope for all failures.
type ErrorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id"`
}
