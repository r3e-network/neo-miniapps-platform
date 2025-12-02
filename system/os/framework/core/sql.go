package service

import (
	"database/sql"
	"time"
)

// SQL helper functions for consistent database handling.
// These consolidate duplicate implementations across store packages.

// PtrTime safely dereferences a time pointer, returning zero time if nil.
func PtrTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// ToNullString converts a string to sql.NullString.
// Empty strings result in a NULL value.
func ToNullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}

// ToNullInt64 converts an int64 to sql.NullInt64.
// Zero values result in a NULL value.
func ToNullInt64(i int64) sql.NullInt64 {
	return sql.NullInt64{
		Int64: i,
		Valid: i != 0,
	}
}

// FromNullString extracts the string value from sql.NullString.
// Returns empty string if NULL.
func FromNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// FromNullInt64 extracts the int64 value from sql.NullInt64.
// Returns 0 if NULL.
func FromNullInt64(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}

// ToNullTime converts a time.Time to sql.NullTime.
// Zero time values result in a NULL value.
func ToNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{
		Time:  t,
		Valid: !t.IsZero(),
	}
}

// FromNullTime extracts the time.Time value from sql.NullTime.
// Returns zero time if NULL.
func FromNullTime(nt sql.NullTime) time.Time {
	if nt.Valid {
		return nt.Time
	}
	return time.Time{}
}
