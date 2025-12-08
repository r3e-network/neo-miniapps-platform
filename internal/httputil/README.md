# HTTPUtil Module

The `httputil` module provides HTTP utility functions for the Service Layer.

## Overview

This module provides common HTTP handling utilities including:

- JSON request/response helpers
- Error response formatting
- Authentication helpers
- Path parameter extraction

## Functions

### JSON Helpers

```go
// Write JSON response
httputil.WriteJSON(w, http.StatusOK, data)

// Decode JSON request body
var input MyInput
if !httputil.DecodeJSON(w, r, &input) {
    return // Error already written
}
```

### Error Responses

```go
// 400 Bad Request
httputil.BadRequest(w, "invalid input")

// 401 Unauthorized
httputil.Unauthorized(w, "authentication required")

// 403 Forbidden
httputil.Forbidden(w, "access denied")

// 404 Not Found
httputil.NotFound(w, "resource not found")

// 500 Internal Server Error
httputil.InternalError(w, "something went wrong")
```

### Authentication Helpers

```go
// Require user ID from header
userID, ok := httputil.RequireUserID(w, r)
if !ok {
    return // 401 already written
}

// Get optional user ID
userID := httputil.GetUserID(r)
```

### Path Parameters

```go
// Extract path parameter
// For path "/request/123/status", extract "123"
id := httputil.PathParam(r.URL.Path, "/request/", "/status")
```

## Response Format

All error responses follow a consistent format:

```json
{
    "error": "error message here"
}
```

Success responses return the data directly:

```json
{
    "id": "123",
    "status": "success",
    ...
}
```

## Usage Example

```go
func (s *Service) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
    // Require authentication
    userID, ok := httputil.RequireUserID(w, r)
    if !ok {
        return
    }

    // Decode request body
    var input CreateRequestInput
    if !httputil.DecodeJSON(w, r, &input) {
        return
    }

    // Validate input
    if input.Amount <= 0 {
        httputil.BadRequest(w, "amount must be positive")
        return
    }

    // Process request...
    result, err := s.processRequest(r.Context(), userID, input)
    if err != nil {
        httputil.InternalError(w, "failed to process request")
        return
    }

    // Return success response
    httputil.WriteJSON(w, http.StatusCreated, result)
}
```

## Headers

| Header | Purpose |
|--------|---------|
| `X-User-ID` | User identifier (set by gateway after auth) |
| `Content-Type` | Must be `application/json` for POST/PUT |
| `Authorization` | Bearer token for API authentication |

## Testing

```bash
go test ./internal/httputil/... -v
```
