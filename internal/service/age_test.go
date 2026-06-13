package service

import (
	"testing"
	"time"
)

// date is a small helper to build a UTC date.
func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestCalculateAge(t *testing.T) {
	tests := []struct {
		name string
		dob  time.Time
		now  time.Time
		want int
	}{
		{
			name: "born today -> age 0",
			dob:  date(2026, time.June, 13),
			now:  date(2026, time.June, 13),
			want: 0,
		},
		{
			name: "exactly 35 years ago today -> age 35",
			dob:  date(1990, time.June, 13),
			now:  date(2025, time.June, 13),
			want: 35,
		},
		{
			name: "birthday not yet this year -> year diff minus 1",
			// Born Dec 31; as of June the birthday hasn't occurred yet.
			dob:  date(1990, time.December, 31),
			now:  date(2025, time.June, 13),
			want: 34, // 2025-1990 = 35, minus 1 = 34
		},
		{
			name: "birthday already passed this year -> year diff",
			// Born Jan 1; by June the birthday has passed.
			dob:  date(1990, time.January, 1),
			now:  date(2025, time.June, 13),
			want: 35,
		},
		{
			name: "birthday is today -> counts as passed",
			dob:  date(2000, time.March, 15),
			now:  date(2025, time.March, 15),
			want: 25,
		},
		{
			name: "day before birthday -> not yet",
			dob:  date(2000, time.March, 15),
			now:  date(2025, time.March, 14),
			want: 24,
		},
		{
			name: "leap year birthday, non-leap year before Feb 29 -> not yet",
			// Born Feb 29 2000. In 2025 (non-leap), as of Feb 28 the birthday
			// has not occurred yet.
			dob:  date(2000, time.February, 29),
			now:  date(2025, time.February, 28),
			want: 24, // 2025-2000 = 25, minus 1
		},
		{
			name: "leap year birthday, on Mar 1 of non-leap year -> occurred",
			dob:  date(2000, time.February, 29),
			now:  date(2025, time.March, 1),
			want: 25,
		},
		{
			name: "leap year birthday, on Feb 29 of a leap year -> exact",
			dob:  date(2000, time.February, 29),
			now:  date(2024, time.February, 29),
			want: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateAge(tt.dob, tt.now)
			if got != tt.want {
				t.Errorf("CalculateAge(%s, %s) = %d, want %d",
					tt.dob.Format("2006-01-02"),
					tt.now.Format("2006-01-02"),
					got, tt.want)
			}
		})
	}
}

// TestCalculateAge_NeverNegative guards against future DOBs producing a
// negative age (the function clamps to 0).
func TestCalculateAge_NeverNegative(t *testing.T) {
	dob := date(2030, time.January, 1)
	now := date(2025, time.June, 13)
	if got := CalculateAge(dob, now); got != 0 {
		t.Errorf("expected clamped age 0 for future dob, got %d", got)
	}
}
