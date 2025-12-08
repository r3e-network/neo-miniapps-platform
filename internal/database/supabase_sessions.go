package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// Session Operations
// =============================================================================

// CreateSession creates a new user session.
func (r *Repository) CreateSession(ctx context.Context, session *UserSession) error {
	data, err := r.client.request(ctx, "POST", "user_sessions", session, "")
	if err != nil {
		return err
	}
	var sessions []UserSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return err
	}
	if len(sessions) > 0 {
		session.ID = sessions[0].ID
	}
	return nil
}

// GetSessionByTokenHash retrieves a session by token hash.
func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*UserSession, error) {
	now := time.Now().Format(time.RFC3339)
	query := fmt.Sprintf("token_hash=eq.%s&expires_at=gt.%s&limit=1", tokenHash, now)
	data, err := r.client.request(ctx, "GET", "user_sessions", nil, query)
	if err != nil {
		return nil, err
	}

	var sessions []UserSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("session not found or expired")
	}
	return &sessions[0], nil
}

// UpdateSessionActivity updates the last_active timestamp.
func (r *Repository) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	update := map[string]interface{}{
		"last_active": time.Now(),
	}
	_, err := r.client.request(ctx, "PATCH", "user_sessions", update, "id=eq."+sessionID)
	return err
}

// DeleteSession deletes a session (logout).
func (r *Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.client.request(ctx, "DELETE", "user_sessions", nil, "token_hash=eq."+tokenHash)
	return err
}

// DeleteUserSessions deletes all sessions for a user.
func (r *Repository) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := r.client.request(ctx, "DELETE", "user_sessions", nil, "user_id=eq."+userID)
	return err
}
