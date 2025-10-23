package random

import "time"

// Result represents a generated random value.
type Result struct {
	Value     []byte
	CreatedAt time.Time
}
