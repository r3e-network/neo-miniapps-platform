package httpapi

import (
	"fmt"
	"strconv"
	"strings"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

func parseLimitParam(raw string, defaultLimit int) (int, error) {
	def := core.DefaultListLimit
	if defaultLimit > 0 {
		def = defaultLimit
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	clamped := core.ClampLimit(parsed, def, core.MaxListLimit)
	return clamped, nil
}
