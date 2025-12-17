package neoflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/infrastructure/runtime"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/automation/supabase"
)

func (s *Service) checkAndExecuteTriggers(ctx context.Context) {
	if s.repo == nil {
		return
	}
	triggers, err := s.repo.GetPendingTriggers(ctx)
	if err != nil {
		return
	}

	now := time.Now()
	for i := range triggers {
		trigger := &triggers[i]
		if !trigger.Enabled {
			continue
		}
		if trigger.TriggerType == "cron" && !trigger.NextExecution.IsZero() {
			if now.After(trigger.NextExecution) {
				go s.executeTrigger(ctx, trigger)
			}
		}
	}
}

func (s *Service) executeTrigger(ctx context.Context, trigger *neoflowsupabase.Trigger) {
	var actionType string
	if len(trigger.Action) > 0 {
		var act Action
		if err := json.Unmarshal(trigger.Action, &act); err == nil {
			actionType = act.Type
		}
	}

	// Execute the action (best-effort)
	err := s.dispatchAction(ctx, trigger.Action)

	// Update last execution and calculate next
	trigger.LastExecution = time.Now()
	if trigger.TriggerType == "cron" && trigger.Schedule != "" {
		next, cronErr := s.parseNextCronExecution(trigger.Schedule)
		if cronErr != nil {
			s.Logger().WithContext(ctx).WithError(cronErr).WithField("trigger_id", trigger.ID).Warn("invalid cron schedule")
			trigger.NextExecution = time.Time{}
		} else {
			trigger.NextExecution = next
		}
	}
	if updateErr := s.repo.UpdateTrigger(ctx, trigger); updateErr != nil {
		s.Logger().WithContext(ctx).WithError(updateErr).WithField("trigger_id", trigger.ID).Warn("failed to update trigger")
	}

	// Persist execution log
	if s.repo != nil {
		exec := &neoflowsupabase.Execution{
			ID:            uuid.New().String(),
			TriggerID:     trigger.ID,
			ExecutedAt:    trigger.LastExecution,
			Success:       err == nil,
			ActionType:    actionType,
			ActionPayload: trigger.Action,
		}
		if err != nil {
			exec.Error = err.Error()
		}
		if execErr := s.repo.CreateExecution(ctx, exec); execErr != nil {
			s.Logger().WithContext(ctx).WithError(execErr).WithField("trigger_id", trigger.ID).Warn("failed to persist execution log")
		}
	}
}

func (s *Service) dispatchAction(ctx context.Context, actionRaw json.RawMessage) error {
	if len(actionRaw) == 0 {
		return nil
	}
	var action Action
	if err := json.Unmarshal(actionRaw, &action); err != nil {
		return err
	}

	switch strings.ToLower(action.Type) {
	case "webhook":
		method := strings.ToUpper(action.Method)
		if method == "" {
			method = http.MethodPost
		}
		if action.URL == "" {
			return fmt.Errorf("webhook url required")
		}

		parsedURL, err := url.Parse(strings.TrimSpace(action.URL))
		if err != nil {
			return fmt.Errorf("invalid webhook url: %w", err)
		}
		scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme))
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("unsupported webhook url scheme: %q", parsedURL.Scheme)
		}
		if parsedURL.Hostname() == "" {
			return fmt.Errorf("webhook url must include hostname")
		}
		if parsedURL.User != nil {
			return fmt.Errorf("webhook url must not include userinfo")
		}

		useMeshClient := isMeshHostname(parsedURL.Hostname())
		strict := runtime.StrictIdentityMode()

		// In strict identity mode, never allow plaintext external webhooks.
		if strict && !useMeshClient && scheme != "https" {
			return fmt.Errorf("external webhook url must use https in strict identity mode")
		}

		// In strict identity mode, never allow internal (mesh) webhooks without mTLS.
		if strict && useMeshClient {
			if m := s.Marble(); m == nil || m.TLSConfig() == nil {
				return fmt.Errorf("mesh webhook requires Marble mTLS in strict identity mode")
			}
		}

		// Mitigate SSRF: in strict identity mode, prevent external webhooks from
		// reaching loopback/link-local/private networks unless explicitly allowed.
		if strict && !useMeshClient && !allowPrivateWebhookTargets() {
			if validateErr := validateWebhookHostname(ctx, parsedURL.Hostname()); validateErr != nil {
				return validateErr
			}
		}

		// If this is an internal (mesh) URL and Marble mTLS is available, enforce
		// HTTPS so peer identity can be verified.
		if m := s.Marble(); m != nil && m.TLSConfig() != nil && useMeshClient {
			parsedURL.Scheme = "https"
		}

		targetURL := parsedURL.String()
		req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(action.Body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		// Use Marble mTLS client only for internal mesh targets. External webhooks
		// must use the system trust store (Marble root CA is not a public CA).
		httpClient := &http.Client{Timeout: 30 * time.Second}
		if m := s.Marble(); m != nil {
			if useMeshClient {
				httpClient = m.HTTPClient()
			} else {
				httpClient = m.ExternalHTTPClient()
			}
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("webhook status %d", resp.StatusCode)
		}
	default:
		// Unknown action type; ignore
	}
	return nil
}

func allowPrivateWebhookTargets() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("NEOFLOW_WEBHOOK_ALLOW_PRIVATE_NETWORKS")))
	return raw == "1" || raw == "true" || raw == "yes"
}

func validateWebhookHostname(ctx context.Context, rawHost string) error {
	host := strings.TrimSpace(rawHost)
	host = strings.TrimSuffix(host, ".")
	hostLower := strings.ToLower(host)

	if hostLower == "" {
		return fmt.Errorf("webhook url must include hostname")
	}
	if hostLower == "localhost" || strings.HasSuffix(hostLower, ".localhost") {
		return fmt.Errorf("external webhook hostname not allowed in strict identity mode")
	}

	if ip := net.ParseIP(hostLower); ip != nil {
		if isDisallowedWebhookIP(ip) {
			return fmt.Errorf("external webhook target IP not allowed in strict identity mode")
		}
		return nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, hostLower)
	if err != nil {
		return fmt.Errorf("failed to resolve webhook hostname: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("failed to resolve webhook hostname: no addresses found")
	}

	for _, addr := range addrs {
		if isDisallowedWebhookIP(addr.IP) {
			return fmt.Errorf("external webhook target resolves to a private or local IP which is not allowed in strict identity mode")
		}
	}
	return nil
}

func isDisallowedWebhookIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip.IsPrivate() {
		return true
	}

	// Carrier-grade NAT (RFC 6598): 100.64.0.0/10
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1]&0xC0 == 0x40 {
			return true
		}
	}

	return false
}

func isMeshHostname(rawHost string) bool {
	host := strings.ToLower(strings.TrimSpace(rawHost))
	if host == "" {
		return false
	}

	if strings.HasSuffix(host, ".svc.cluster.local") ||
		strings.HasSuffix(host, ".service-layer") ||
		strings.HasSuffix(host, ".service-layer.svc.cluster.local") {
		return true
	}

	switch host {
	case "gateway",
		"globalsigner",
		"neofeeds",
		"neoflow",
		"neoaccounts",
		"accountpool",
		"neocompute",
		"neooracle",
		"oracle":
		return true
	default:
		return false
	}
}

// parseNextCronExecution parses a cron expression and returns the next execution time.
// Supports standard 5-field cron: minute hour day-of-month month day-of-week
// Supports: specific values (5), wildcards (*), ranges (1-5), lists (1,3,5), steps (*/15)
func (s *Service) parseNextCronExecution(cronExpr string) (time.Time, error) {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return time.Time{}, fmt.Errorf("invalid cron expression: expected 5 fields")
	}

	// Parse each field into allowed values
	minutes, err := parseCronField(parts[0], 0, 59)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute field: %w", err)
	}
	hours, err := parseCronField(parts[1], 0, 23)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour field: %w", err)
	}
	days, err := parseCronField(parts[2], 1, 31)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day field: %w", err)
	}
	months, err := parseCronField(parts[3], 1, 12)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month field: %w", err)
	}
	weekdays, err := parseCronField(parts[4], 0, 6)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid weekday field: %w", err)
	}

	// Find next matching time (search up to 1 year ahead)
	now := time.Now()
	candidate := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
	candidate = candidate.Add(time.Minute) // Start from next minute

	maxIterations := 525600 // 1 year in minutes
	for i := 0; i < maxIterations; i++ {
		if months[int(candidate.Month())] &&
			days[candidate.Day()] &&
			weekdays[int(candidate.Weekday())] &&
			hours[candidate.Hour()] &&
			minutes[candidate.Minute()] {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, fmt.Errorf("no matching time found within 1 year")
}

// parseCronField parses a single cron field and returns a map of allowed values.
func parseCronField(field string, minValue, maxValue int) (map[int]bool, error) {
	allowed := make(map[int]bool)

	// Handle wildcard
	if field == "*" {
		for i := minValue; i <= maxValue; i++ {
			allowed[i] = true
		}
		return allowed, nil
	}

	// Handle step with wildcard (*/n)
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step: %s", field)
		}
		for i := minValue; i <= maxValue; i += step {
			allowed[i] = true
		}
		return allowed, nil
	}

	// Handle comma-separated list
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// Handle range (n-m) or range with step (n-m/s)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "/")
			rangeStr := rangeParts[0]
			step := 1
			if len(rangeParts) == 2 {
				var err error
				step, err = strconv.Atoi(rangeParts[1])
				if err != nil || step <= 0 {
					return nil, fmt.Errorf("invalid step in range: %s", part)
				}
			}

			bounds := strings.Split(rangeStr, "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err1 := strconv.Atoi(bounds[0])
			end, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil || start < minValue || end > maxValue || start > end {
				return nil, fmt.Errorf("invalid range values: %s", part)
			}
			for i := start; i <= end; i += step {
				allowed[i] = true
			}
		} else {
			// Single value
			val, err := strconv.Atoi(part)
			if err != nil || val < minValue || val > maxValue {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			allowed[val] = true
		}
	}

	return allowed, nil
}

// Note: platform-anchored automation tasks live in anchored_tasks.go.
