package httputil

import (
	"fmt"
	"io"
)

// BodyTooLargeError is returned by ReadAllStrict when the body exceeds the limit.
type BodyTooLargeError struct {
	Limit int64
}

func (e *BodyTooLargeError) Error() string {
	return fmt.Sprintf("body exceeds limit of %d bytes", e.Limit)
}

// ReadAllWithLimit reads up to limit bytes from r.
// It returns the bytes read, whether the body exceeded the limit, and any I/O error.
//
// This is useful for logging or building error messages without risking OOM.
func ReadAllWithLimit(r io.Reader, limit int64) (body []byte, truncated bool, err error) {
	if limit <= 0 {
		return nil, false, fmt.Errorf("limit must be positive")
	}
	if r == nil {
		return nil, false, fmt.Errorf("reader is nil")
	}
	limited := io.LimitReader(r, limit+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if int64(len(b)) > limit {
		return b[:limit], true, nil
	}
	return b, false, nil
}

// ReadAllStrict reads the full body from r up to limit bytes.
// If the body exceeds limit, it returns a *BodyTooLargeError.
func ReadAllStrict(r io.Reader, limit int64) ([]byte, error) {
	b, truncated, err := ReadAllWithLimit(r, limit)
	if err != nil {
		return nil, err
	}
	if truncated {
		return nil, &BodyTooLargeError{Limit: limit}
	}
	return b, nil
}
