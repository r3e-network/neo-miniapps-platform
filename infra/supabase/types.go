// Package supabase provides a comprehensive TEE-aware Supabase client.
// All API keys are stored in the TEE vault and injected into requests inside the enclave.
package supabase

import (
	"time"
)

// =============================================================================
// Configuration
// =============================================================================

// Config holds Supabase client configuration.
type Config struct {
	// ProjectURL is the Supabase project URL (e.g., https://xxx.supabase.co)
	ProjectURL string

	// AnonKeySecret is the secret name in TEE vault for the anon key
	AnonKeySecret string

	// ServiceKeySecret is the secret name in TEE vault for the service role key
	// Used for admin operations that bypass RLS
	ServiceKeySecret string

	// JWTSecret is the secret name in TEE vault for JWT verification
	JWTSecret string

	// AllowedHosts restricts outbound requests (derived from ProjectURL if empty)
	AllowedHosts []string

	// DefaultHeaders are added to every request
	DefaultHeaders map[string]string

	// Timeout for HTTP requests
	Timeout time.Duration
}

// =============================================================================
// Auth Types
// =============================================================================

// User represents a Supabase user.
type User struct {
	ID               string                 `json:"id"`
	Aud              string                 `json:"aud"`
	Role             string                 `json:"role"`
	Email            string                 `json:"email"`
	EmailConfirmedAt *time.Time             `json:"email_confirmed_at,omitempty"`
	Phone            string                 `json:"phone,omitempty"`
	PhoneConfirmedAt *time.Time             `json:"phone_confirmed_at,omitempty"`
	ConfirmedAt      *time.Time             `json:"confirmed_at,omitempty"`
	LastSignInAt     *time.Time             `json:"last_sign_in_at,omitempty"`
	AppMetadata      map[string]interface{} `json:"app_metadata,omitempty"`
	UserMetadata     map[string]interface{} `json:"user_metadata,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// Session represents an auth session.
type Session struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user,omitempty"`
}

// SignUpRequest for user registration.
type SignUpRequest struct {
	Email    string                 `json:"email"`
	Password string                 `json:"password"`
	Phone    string                 `json:"phone,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// SignInRequest for user authentication.
type SignInRequest struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Phone    string `json:"phone,omitempty"`
}

// TokenClaims represents JWT claims.
type TokenClaims struct {
	Sub       string                 `json:"sub"`
	Aud       string                 `json:"aud"`
	Role      string                 `json:"role"`
	Email     string                 `json:"email,omitempty"`
	Phone     string                 `json:"phone,omitempty"`
	AppMeta   map[string]interface{} `json:"app_metadata,omitempty"`
	UserMeta  map[string]interface{} `json:"user_metadata,omitempty"`
	IssuedAt  int64                  `json:"iat"`
	ExpiresAt int64                  `json:"exp"`
}

// =============================================================================
// Database Types
// =============================================================================

// QueryResult represents a database query result.
type QueryResult struct {
	Data       []byte `json:"data"`
	Count      *int64 `json:"count,omitempty"`
	StatusCode int    `json:"status_code"`
}

// FilterOperator for query filters.
type FilterOperator string

const (
	OpEq     FilterOperator = "eq"
	OpNeq    FilterOperator = "neq"
	OpGt     FilterOperator = "gt"
	OpGte    FilterOperator = "gte"
	OpLt     FilterOperator = "lt"
	OpLte    FilterOperator = "lte"
	OpLike   FilterOperator = "like"
	OpILike  FilterOperator = "ilike"
	OpIs     FilterOperator = "is"
	OpIn     FilterOperator = "in"
	OpContains FilterOperator = "cs"
	OpContainedBy FilterOperator = "cd"
	OpOverlap FilterOperator = "ov"
)

// OrderDirection for sorting.
type OrderDirection string

const (
	OrderAsc  OrderDirection = "asc"
	OrderDesc OrderDirection = "desc"
)

// =============================================================================
// Storage Types
// =============================================================================

// Bucket represents a storage bucket.
type Bucket struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Owner     string     `json:"owner,omitempty"`
	Public    bool       `json:"public"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// FileObject represents a file in storage.
type FileObject struct {
	Name      string                 `json:"name"`
	ID        string                 `json:"id,omitempty"`
	BucketID  string                 `json:"bucket_id,omitempty"`
	Owner     string                 `json:"owner,omitempty"`
	CreatedAt *time.Time             `json:"created_at,omitempty"`
	UpdatedAt *time.Time             `json:"updated_at,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// UploadOptions for file uploads.
type UploadOptions struct {
	ContentType string
	CacheControl string
	Upsert      bool
}

// =============================================================================
// Realtime Types
// =============================================================================

// RealtimeEvent represents a realtime event.
type RealtimeEvent struct {
	Type      string                 `json:"type"`
	Table     string                 `json:"table"`
	Schema    string                 `json:"schema"`
	Record    map[string]interface{} `json:"record,omitempty"`
	OldRecord map[string]interface{} `json:"old_record,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// RealtimeEventType for subscription filtering.
type RealtimeEventType string

const (
	EventInsert RealtimeEventType = "INSERT"
	EventUpdate RealtimeEventType = "UPDATE"
	EventDelete RealtimeEventType = "DELETE"
	EventAll    RealtimeEventType = "*"
)

// SubscriptionConfig for realtime subscriptions.
type SubscriptionConfig struct {
	Schema string
	Table  string
	Event  RealtimeEventType
	Filter string // Optional PostgREST filter
}

// =============================================================================
// Error Types
// =============================================================================

// Error represents a Supabase API error.
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
	Hint       string `json:"hint,omitempty"`
	StatusCode int    `json:"status_code"`
}

func (e *Error) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}

// NewError creates a new Supabase error.
func NewError(code, message string, statusCode int) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Common errors
var (
	ErrUnauthorized     = NewError("unauthorized", "unauthorized", 401)
	ErrForbidden        = NewError("forbidden", "forbidden", 403)
	ErrNotFound         = NewError("not_found", "resource not found", 404)
	ErrConflict         = NewError("conflict", "resource already exists", 409)
	ErrInternalError    = NewError("internal_error", "internal server error", 500)
)
