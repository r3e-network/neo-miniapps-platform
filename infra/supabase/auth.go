package supabase

import (
	"context"
	"encoding/json"
	"fmt"
)

// AuthClient handles Supabase Auth operations.
// JWT verification happens inside the TEE enclave.
type AuthClient struct {
	client *Client
}

// SignUp creates a new user.
func (a *AuthClient) SignUp(ctx context.Context, req SignUpRequest) (*Session, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := a.client.request(ctx, "POST", a.client.authURL+"/signup", body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &session, nil
}

// SignInWithPassword authenticates a user with email/password.
func (a *AuthClient) SignInWithPassword(ctx context.Context, email, password string) (*Session, error) {
	req := map[string]string{
		"email":    email,
		"password": password,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := a.client.request(ctx, "POST", a.client.authURL+"/token?grant_type=password", body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &session, nil
}

// SignInWithPhone authenticates a user with phone/password.
func (a *AuthClient) SignInWithPhone(ctx context.Context, phone, password string) (*Session, error) {
	req := map[string]string{
		"phone":    phone,
		"password": password,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := a.client.request(ctx, "POST", a.client.authURL+"/token?grant_type=password", body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &session, nil
}

// RefreshToken refreshes an access token.
func (a *AuthClient) RefreshToken(ctx context.Context, refreshToken string) (*Session, error) {
	req := map[string]string{
		"refresh_token": refreshToken,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := a.client.request(ctx, "POST", a.client.authURL+"/token?grant_type=refresh_token", body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &session, nil
}

// GetUser retrieves the current user using an access token.
func (a *AuthClient) GetUser(ctx context.Context, accessToken string) (*User, error) {
	respBody, statusCode, err := a.client.requestWithToken(ctx, "GET", a.client.authURL+"/user", nil, nil, accessToken)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var user User
	if err := json.Unmarshal(respBody, &user); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &user, nil
}

// UpdateUser updates the current user.
func (a *AuthClient) UpdateUser(ctx context.Context, accessToken string, updates map[string]interface{}) (*User, error) {
	body, err := json.Marshal(updates)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := a.client.requestWithToken(ctx, "PUT", a.client.authURL+"/user", body, nil, accessToken)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var user User
	if err := json.Unmarshal(respBody, &user); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &user, nil
}

// SignOut signs out a user.
func (a *AuthClient) SignOut(ctx context.Context, accessToken string) error {
	_, statusCode, err := a.client.requestWithToken(ctx, "POST", a.client.authURL+"/logout", nil, nil, accessToken)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return fmt.Errorf("sign out failed with status %d", statusCode)
	}

	return nil
}

// ResetPasswordForEmail sends a password reset email.
func (a *AuthClient) ResetPasswordForEmail(ctx context.Context, email string) error {
	req := map[string]string{
		"email": email,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := a.client.request(ctx, "POST", a.client.authURL+"/recover", body, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return parseError(respBody, statusCode)
	}

	return nil
}

// VerifyOTP verifies an OTP token.
func (a *AuthClient) VerifyOTP(ctx context.Context, email, token, otpType string) (*Session, error) {
	req := map[string]string{
		"email": email,
		"token": token,
		"type":  otpType,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := a.client.request(ctx, "POST", a.client.authURL+"/verify", body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var session Session
	if err := json.Unmarshal(respBody, &session); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &session, nil
}

// =============================================================================
// Admin Operations (require service role key)
// =============================================================================

// AdminGetUser retrieves a user by ID (admin operation).
func (a *AuthClient) AdminGetUser(ctx context.Context, userID string) (*User, error) {
	respBody, statusCode, err := a.client.requestWithServiceKey(ctx, "GET", a.client.authURL+"/admin/users/"+userID, nil, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var user User
	if err := json.Unmarshal(respBody, &user); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &user, nil
}

// AdminListUsers lists all users (admin operation).
func (a *AuthClient) AdminListUsers(ctx context.Context, page, perPage int) ([]User, error) {
	url := fmt.Sprintf("%s/admin/users?page=%d&per_page=%d", a.client.authURL, page, perPage)

	respBody, statusCode, err := a.client.requestWithServiceKey(ctx, "GET", url, nil, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var result struct {
		Users []User `json:"users"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Users, nil
}

// AdminDeleteUser deletes a user (admin operation).
func (a *AuthClient) AdminDeleteUser(ctx context.Context, userID string) error {
	respBody, statusCode, err := a.client.requestWithServiceKey(ctx, "DELETE", a.client.authURL+"/admin/users/"+userID, nil, nil)
	if err != nil {
		return err
	}

	if statusCode >= 400 {
		return parseError(respBody, statusCode)
	}

	return nil
}

// AdminUpdateUser updates a user (admin operation).
func (a *AuthClient) AdminUpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*User, error) {
	body, err := json.Marshal(updates)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := a.client.requestWithServiceKey(ctx, "PUT", a.client.authURL+"/admin/users/"+userID, body, nil)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, parseError(respBody, statusCode)
	}

	var user User
	if err := json.Unmarshal(respBody, &user); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &user, nil
}
