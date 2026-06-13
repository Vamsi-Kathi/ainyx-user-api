package service

import (
	"errors"
	"testing"
	"time"
)

// withFixedNow temporarily overrides the package clock for deterministic tests
// and restores it afterward.
func withFixedNow(t *testing.T, fixed time.Time) {
	t.Helper()
	orig := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = orig })
}

func TestParseDOB(t *testing.T) {
	// Pin "now" so future/age boundaries are deterministic.
	withFixedNow(t, date(2026, time.June, 13))

	tests := []struct {
		name    string
		input   string
		wantErr error
		wantOK  bool
	}{
		{
			name:   "valid past date, adult",
			input:  "1990-05-10",
			wantOK: true,
		},
		{
			name:   "valid, exactly 1 year old yesterday's-style boundary",
			input:  "2025-01-01",
			wantOK: true,
		},
		{
			name:    "bad format - slashes",
			input:   "1990/05/10",
			wantErr: ErrInvalidDOB,
		},
		{
			name:    "bad format - includes time",
			input:   "1990-05-10T00:00:00Z",
			wantErr: ErrInvalidDOB,
		},
		{
			name:    "bad format - out of range month",
			input:   "1990-13-10",
			wantErr: ErrInvalidDOB,
		},
		{
			name:    "bad format - empty",
			input:   "",
			wantErr: ErrInvalidDOB,
		},
		{
			name:    "future date",
			input:   "2030-01-01",
			wantErr: ErrFutureDOB,
		},
		{
			name:    "today is not in the past",
			input:   "2026-06-13",
			wantErr: ErrFutureDOB,
		},
		{
			name:    "before 1900",
			input:   "1899-12-31",
			wantErr: ErrDOBTooOld,
		},
		{
			name:    "less than 1 year old",
			input:   "2026-01-01",
			wantErr: ErrTooYoung,
		},
		{
			name:   "exactly 1900-01-01 is allowed",
			input:  "1900-01-01",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDOB(tt.input)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("parseDOB(%q) unexpected error: %v", tt.input, err)
				}
				if got.IsZero() {
					t.Fatalf("parseDOB(%q) returned zero time on success", tt.input)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("parseDOB(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// TestDOBErrorMessages locks the exact user-facing strings required by the spec.
func TestDOBErrorMessages(t *testing.T) {
	cases := map[error]string{
		ErrInvalidDOB: "use format YYYY-MM-DD",
		ErrFutureDOB:  "date of birth cannot be in the future",
		ErrDOBTooOld:  "date of birth seems invalid",
		ErrTooYoung:   "person must be at least 1 year old",
		ErrNotFound:   "user not found",
	}
	for err, want := range cases {
		if err.Error() != want {
			t.Errorf("message = %q, want %q", err.Error(), want)
		}
	}
}
