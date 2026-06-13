package service

import "time"

// CalculateAge returns the integer age in completed years between dob and now.
//
// Rule: if the person's birthday has not yet occurred in the "now" year, we
// subtract one from the raw year difference. This correctly handles leap-year
// birthdays (Feb 29): in non-leap years the birthday is treated as not-yet
// occurred until March 1, which matches Go's calendar comparison semantics.
//
// It is a pure function (now is injected) so it can be unit tested
// deterministically.
func CalculateAge(dob, now time.Time) int {
	// Normalize both to UTC date components to avoid timezone drift.
	dob = dob.UTC()
	now = now.UTC()

	age := now.Year() - dob.Year()

	// Has the birthday happened yet this year?
	if now.Month() < dob.Month() ||
		(now.Month() == dob.Month() && now.Day() < dob.Day()) {
		age--
	}

	if age < 0 {
		age = 0
	}
	return age
}
