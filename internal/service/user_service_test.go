package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	db "github.com/Vamsi-Kathi/ainyx-user-api/db/sqlc"
	"github.com/Vamsi-Kathi/ainyx-user-api/internal/models"
)

// fakeQuerier is an in-test implementation of db.Querier. Each method delegates
// to a function field so individual tests can program exactly the behavior they
// need; unset fields panic loudly if hit unexpectedly.
type fakeQuerier struct {
	createUser func(context.Context, db.CreateUserParams) (sql.Result, error)
	getUser    func(context.Context, uint64) (db.User, error)
	listUsers  func(context.Context, db.ListUsersParams) ([]db.User, error)
	countUsers func(context.Context) (int64, error)
	updateUser func(context.Context, db.UpdateUserParams) (sql.Result, error)
	deleteUser func(context.Context, uint64) (sql.Result, error)
}

func (f *fakeQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (sql.Result, error) {
	return f.createUser(ctx, arg)
}
func (f *fakeQuerier) GetUser(ctx context.Context, id uint64) (db.User, error) {
	return f.getUser(ctx, id)
}
func (f *fakeQuerier) ListUsers(ctx context.Context, arg db.ListUsersParams) ([]db.User, error) {
	return f.listUsers(ctx, arg)
}
func (f *fakeQuerier) CountUsers(ctx context.Context) (int64, error) {
	return f.countUsers(ctx)
}
func (f *fakeQuerier) UpdateUser(ctx context.Context, arg db.UpdateUserParams) (sql.Result, error) {
	return f.updateUser(ctx, arg)
}
func (f *fakeQuerier) DeleteUser(ctx context.Context, id uint64) (sql.Result, error) {
	return f.deleteUser(ctx, id)
}

// fakeResult is a programmable sql.Result.
type fakeResult struct {
	lastID, affected   int64
	lastErr, affectErr error
}

func (r fakeResult) LastInsertId() (int64, error) { return r.lastID, r.lastErr }
func (r fakeResult) RowsAffected() (int64, error) { return r.affected, r.affectErr }

func newService(q db.Querier) *UserService {
	return NewUserService(q, zap.NewNop())
}

func TestService_Create(t *testing.T) {
	withFixedNow(t, date(2026, time.June, 13))

	var captured db.CreateUserParams
	q := &fakeQuerier{
		createUser: func(_ context.Context, arg db.CreateUserParams) (sql.Result, error) {
			captured = arg
			return fakeResult{lastID: 42}, nil
		},
	}

	resp, err := newService(q).Create(context.Background(), models.CreateUserRequest{
		Name: "Ada Lovelace",
		Dob:  "1990-05-10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 42 || resp.Name != "Ada Lovelace" || resp.Dob != "1990-05-10" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	// The parsed DOB must be persisted as a real date, not the raw string.
	if want := date(1990, time.May, 10); !captured.Dob.Equal(want) {
		t.Fatalf("persisted dob = %v, want %v", captured.Dob, want)
	}
}

func TestService_Create_InvalidDOBShortCircuits(t *testing.T) {
	withFixedNow(t, date(2026, time.June, 13))

	q := &fakeQuerier{
		createUser: func(context.Context, db.CreateUserParams) (sql.Result, error) {
			t.Fatal("CreateUser must not be called when DOB is invalid")
			return nil, nil
		},
	}

	_, err := newService(q).Create(context.Background(), models.CreateUserRequest{
		Name: "Future Kid",
		Dob:  "2030-01-01",
	})
	if !errors.Is(err, ErrFutureDOB) {
		t.Fatalf("err = %v, want ErrFutureDOB", err)
	}
}

func TestService_Get(t *testing.T) {
	withFixedNow(t, date(2025, time.June, 13))

	q := &fakeQuerier{
		getUser: func(_ context.Context, id uint64) (db.User, error) {
			return db.User{ID: id, Name: "Grace", Dob: date(1990, time.June, 13)}, nil
		},
	}

	resp, err := newService(q).Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 7 || resp.Name != "Grace" || resp.Dob != "1990-06-13" || resp.Age != 35 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	q := &fakeQuerier{
		getUser: func(context.Context, uint64) (db.User, error) {
			return db.User{}, sql.ErrNoRows
		},
	}
	if _, err := newService(q).Get(context.Background(), 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestService_List_PaginationClamping(t *testing.T) {
	tests := []struct {
		name                        string
		page, limit                 int
		wantLimit, wantOffset       int32
		wantRespPage, wantRespLimit int
	}{
		{"defaults applied for zero", 0, 0, 10, 0, 1, 10},
		{"negatives clamped to defaults", -3, -9, 10, 0, 1, 10},
		{"limit capped at 100", 2, 500, 100, 100, 2, 100},
		{"normal offset math", 3, 20, 20, 40, 3, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got db.ListUsersParams
			q := &fakeQuerier{
				listUsers: func(_ context.Context, arg db.ListUsersParams) ([]db.User, error) {
					got = arg
					return []db.User{}, nil
				},
				countUsers: func(context.Context) (int64, error) { return 0, nil },
			}

			resp, err := newService(q).List(context.Background(), tt.page, tt.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Limit != tt.wantLimit || got.Offset != tt.wantOffset {
				t.Fatalf("query params = (limit %d, offset %d), want (limit %d, offset %d)",
					got.Limit, got.Offset, tt.wantLimit, tt.wantOffset)
			}
			if resp.Pagination.Page != tt.wantRespPage || resp.Pagination.Limit != tt.wantRespLimit {
				t.Fatalf("pagination = %+v, want page %d limit %d",
					resp.Pagination, tt.wantRespPage, tt.wantRespLimit)
			}
		})
	}
}

func TestService_List_MapsRowsWithAge(t *testing.T) {
	withFixedNow(t, date(2025, time.June, 13))

	q := &fakeQuerier{
		listUsers: func(context.Context, db.ListUsersParams) ([]db.User, error) {
			return []db.User{
				{ID: 1, Name: "A", Dob: date(2000, time.January, 1)},
				{ID: 2, Name: "B", Dob: date(2010, time.December, 31)},
			}, nil
		},
		countUsers: func(context.Context) (int64, error) { return 2, nil },
	}

	resp, err := newService(q).List(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 2 || resp.Pagination.Total != 2 {
		t.Fatalf("unexpected list: %+v", resp)
	}
	if resp.Data[0].Age != 25 { // born Jan 2000, birthday passed
		t.Errorf("row 0 age = %d, want 25", resp.Data[0].Age)
	}
	if resp.Data[1].Age != 14 { // born Dec 2010, birthday not yet in June
		t.Errorf("row 1 age = %d, want 14", resp.Data[1].Age)
	}
}

func TestService_Update(t *testing.T) {
	withFixedNow(t, date(2026, time.June, 13))

	t.Run("success when a row is matched", func(t *testing.T) {
		q := &fakeQuerier{
			updateUser: func(context.Context, db.UpdateUserParams) (sql.Result, error) {
				return fakeResult{affected: 1}, nil
			},
		}
		resp, err := newService(q).Update(context.Background(), 5, models.UpdateUserRequest{
			Name: "Renamed", Dob: "1991-02-03",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.ID != 5 || resp.Name != "Renamed" {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("not found when no row is matched", func(t *testing.T) {
		q := &fakeQuerier{
			updateUser: func(context.Context, db.UpdateUserParams) (sql.Result, error) {
				return fakeResult{affected: 0}, nil
			},
		}
		_, err := newService(q).Update(context.Background(), 5, models.UpdateUserRequest{
			Name: "Nobody", Dob: "1991-02-03",
		})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("invalid DOB short-circuits before the DB", func(t *testing.T) {
		q := &fakeQuerier{
			updateUser: func(context.Context, db.UpdateUserParams) (sql.Result, error) {
				t.Fatal("UpdateUser must not be called on invalid DOB")
				return nil, nil
			},
		}
		_, err := newService(q).Update(context.Background(), 5, models.UpdateUserRequest{
			Name: "X", Dob: "not-a-date",
		})
		if !errors.Is(err, ErrInvalidDOB) {
			t.Fatalf("err = %v, want ErrInvalidDOB", err)
		}
	})
}

func TestService_Delete(t *testing.T) {
	t.Run("success when a row is removed", func(t *testing.T) {
		q := &fakeQuerier{
			deleteUser: func(context.Context, uint64) (sql.Result, error) {
				return fakeResult{affected: 1}, nil
			},
		}
		if err := newService(q).Delete(context.Background(), 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not found when nothing is removed", func(t *testing.T) {
		q := &fakeQuerier{
			deleteUser: func(context.Context, uint64) (sql.Result, error) {
				return fakeResult{affected: 0}, nil
			},
		}
		if err := newService(q).Delete(context.Background(), 5); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
}
