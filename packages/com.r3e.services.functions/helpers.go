package functions

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func clonePayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		if payload == nil {
			return nil
		}
		return map[string]any{}
	}
	dup := make(map[string]any, len(payload))
	for k, v := range payload {
		dup[k] = cloneValue(v)
	}
	return dup
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return clonePayload(v)
	case []any:
		if len(v) == 0 {
			return []any{}
		}
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneValue(item)
		}
		return out
	case []byte:
		if v == nil {
			return []byte(nil)
		}
		out := make([]byte, len(v))
		copy(out, v)
		return out
	default:
		return v
	}
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	dup := make([]string, len(items))
	copy(dup, items)
	return dup
}

func cloneActionResults(actions []ActionResult) []ActionResult {
	if len(actions) == 0 {
		return nil
	}
	copied := make([]ActionResult, len(actions))
	for i, a := range actions {
		copied[i] = ActionResult{
			Action: Action{
				ID:     a.ID,
				Type:   a.Type,
				Params: clonePayload(a.Params),
			},
			Status: a.Status,
			Result: clonePayload(a.Result),
			Error:  a.Error,
			Meta:   clonePayload(a.Meta),
		}
	}
	return copied
}

func stringParam(params map[string]any, key, fallback string) string {
	if params == nil {
		return fallback
	}
	if value, ok := params[key]; ok {
		switch v := value.(type) {
		case string:
			return v
		case fmt.Stringer:
			return v.String()
		default:
			return fmt.Sprint(v)
		}
	}
	return fallback
}

func stringSliceParam(params map[string]any, key string) []string {
	if params == nil {
		return nil
	}
	value, ok := params[key]
	if !ok {
		return nil
	}
	var list []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" {
			list = append(list, v)
		}
	}
	switch v := value.(type) {
	case []string:
		for _, item := range v {
			add(item)
		}
	case []any:
		for _, item := range v {
			add(fmt.Sprint(item))
		}
	case string:
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ';'
		})
		for _, item := range parts {
			add(item)
		}
	default:
		add(fmt.Sprint(v))
	}
	if len(list) == 0 {
		return nil
	}
	return list
}

func floatParam(params map[string]any, key string) (float64, error) {
	if params == nil {
		return 0, fmt.Errorf("missing %s", key)
	}
	value, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	case string:
		return strconv.ParseFloat(strings.TrimSpace(v), 64)
	default:
		return 0, fmt.Errorf("unsupported number type %T", v)
	}
}

func boolParam(params map[string]any, key string, fallback bool) bool {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	case int:
		return v != 0
	default:
		return fallback
	}
}

func intParam(params map[string]any, key string, fallback int) int {
	if params == nil {
		return fallback
	}
	value, ok := params[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func mapStringStringParam(params map[string]any, key string) (map[string]string, error) {
	if params == nil {
		return nil, nil
	}
	value, ok := params[key]
	if !ok || value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case map[string]string:
		copyMap := make(map[string]string, len(v))
		for k, val := range v {
			copyMap[k] = val
		}
		return copyMap, nil
	case map[string]any:
		copyMap := make(map[string]string, len(v))
		for k, val := range v {
			copyMap[k] = fmt.Sprint(val)
		}
		return copyMap, nil
	default:
		return nil, fmt.Errorf("expected map for %s, got %T", key, value)
	}
}

func stringOrJSON(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
}

func structToMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{
			"error": fmt.Sprintf("marshal: %v", err),
		}
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{
			"error": fmt.Sprintf("unmarshal: %v", err),
		}
	}
	return result
}
