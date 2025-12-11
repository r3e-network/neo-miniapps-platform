package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// =============================================================================
// Session Operations
// =============================================================================

// CreateSession creates a new user session.
func (r *Repository) CreateSession(ctx context.Context, session *UserSession) error {
	if session == nil {
		return fmt.Errorf("%w: session cannot be nil", ErrInvalidInput)
	}
	if err := ValidateUserID(session.UserID); err != nil {
		return err
	}

	data, err := r.client.request(ctx, "POST", "user_sessions", session, "")
	if err != nil {
		return fmt.Errorf("%w: create session: %v", ErrDatabaseError, err)
	}
	var sessions []UserSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return fmt.Errorf("%w: unmarshal sessions: %v", ErrDatabaseError, err)
	}
	if len(sessions) > 0 {
		session.ID = sessions[0].ID
	}
	return nil
}

// GetSessionByTokenHash retrieves a session by token hash.
func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*UserSession, error) {
	if tokenHash == "" {
		return nil, fmt.Errorf("%w: token_hash cannot be empty", ErrInvalidInput)
	}
	tokenHash = SanitizeString(tokenHash)

	now := time.Now().Format(time.RFC3339)
	query := fmt.Sprintf("token_hash=eq.%s&expires_at=gt.%s&limit=1", url.QueryEscape(tokenHash), url.QueryEscape(now))
	data, err := r.client.request(ctx, "GET", "user_sessions", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get session by token hash: %v", ErrDatabaseError, err)
	}

	var sessions []UserSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("%w: unmarshal sessions: %v", ErrDatabaseError, err)
	}
	if len(sessions) == 0 {
		return nil, NewNotFoundError("session", tokenHash)
	}
	return &sessions[0], nil
}

// UpdateSessionActivity updates the last_active timestamp.
func (r *Repository) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	if err := ValidateID(sessionID); err != nil {
		return err
	}

	update := map[string]interface{}{
		"last_active": time.Now(),
	}
	_, err := r.client.request(ctx, "PATCH", "user_sessions", update, "id=eq."+url.QueryEscape(sessionID))
	if err != nil {
		return fmt.Errorf("%w: update session activity: %v", ErrDatabaseError, err)
	}
	return nil
}

// DeleteSession deletes a session (logout).
func (r *Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	if tokenHash == "" {
		return fmt.Errorf("%w: token_hash cannot be empty", ErrInvalidInput)
	}
	tokenHash = SanitizeString(tokenHash)

	_, err := r.client.request(ctx, "DELETE", "user_sessions", nil, "token_hash=eq."+url.QueryEscape(tokenHash))
	if err != nil {
		return fmt.Errorf("%w: delete session: %v", ErrDatabaseError, err)
	}
	return nil
}

// DeleteUserSessions deletes all sessions for a user.
func (r *Repository) DeleteUserSessions(ctx context.Context, userID string) error {
	if err := ValidateUserID(userID); err != nil {
		return err
	}

	_, err := r.client.request(ctx, "DELETE", "user_sessions", nil, "user_id=eq."+url.QueryEscape(userID))
	if err != nil {
		return fmt.Errorf("%w: delete user sessions: %v", ErrDatabaseError, err)
	}
	return nil
}
