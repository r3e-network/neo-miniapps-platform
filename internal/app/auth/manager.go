package auth

import (
	"errors"
	"fmt"
	"sync"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/sha3"
)

type User struct {
	Username string
	Password string
	Role     string
}

var ErrUnauthorised = errors.New("unauthorised")

type Claims struct {
	Username string `json:"sub"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret     []byte
	users      map[string]User
	challenges map[string]challenge
	enforceNeo bool
	mu         sync.Mutex
}

type challenge struct {
	Nonce     string
	ExpiresAt time.Time
}

// NewManager builds a JWT-backed auth manager. The secret must be non-empty to issue tokens.
func NewManager(secret string, users []User) *Manager {
	userMap := make(map[string]User)
	for _, u := range users {
		u.Username = strings.TrimSpace(u.Username)
		if u.Username == "" {
			continue
		}
		if u.Role == "" {
			u.Role = "user"
		}
		userMap[strings.ToLower(u.Username)] = u
	}
	return &Manager{
		secret:     []byte(strings.TrimSpace(secret)),
		users:      userMap,
		challenges: make(map[string]challenge),
		enforceNeo: true,
	}
}

// HasUsers reports whether any user is configured.
func (m *Manager) HasUsers() bool {
	return len(m.users) > 0 && len(m.secret) > 0
}

// Authenticate returns the user if username/password match.
func (m *Manager) Authenticate(username, password string) (User, error) {
	u, ok := m.users[strings.ToLower(strings.TrimSpace(username))]
	if !ok || strings.TrimSpace(password) == "" || u.Password != password {
		return User{}, errors.New("invalid credentials")
	}
	return u, nil
}

// Issue returns a signed JWT for the provided user.
func (m *Manager) Issue(user User, ttl time.Duration) (string, time.Time, error) {
	if len(m.secret) == 0 {
		return "", time.Time{}, errors.New("jwt secret not configured")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	exp := time.Now().Add(ttl)
	claims := Claims{
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.Username,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	return signed, exp, err
}

// Validate parses and validates a JWT token.
func (m *Manager) Validate(tokenString string) (*Claims, error) {
	if len(m.secret) == 0 {
		return nil, errors.New("jwt secret not configured")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// IssueWalletChallenge creates a time-limited nonce for a wallet address.
func (m *Manager) IssueWalletChallenge(wallet string, ttl time.Duration) (string, time.Time, error) {
	if len(m.secret) == 0 {
		return "", time.Time{}, errors.New("jwt secret not configured")
	}
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		return "", time.Time{}, errors.New("wallet required")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	exp := time.Now().Add(ttl)
	nonce := deriveNonce(wallet, m.secret, exp)
	m.mu.Lock()
	m.challenges[strings.ToLower(wallet)] = challenge{Nonce: nonce, ExpiresAt: exp}
	m.mu.Unlock()
	return nonce, exp, nil
}

// VerifyWalletSignature validates an HMAC-based signature over the nonce.
// Clients must compute HMAC-SHA3-256(secret, nonce) hex-encoded.
func (m *Manager) VerifyWalletSignature(wallet, signature, pubKey string) (User, error) {
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		return User{}, errors.New("wallet required")
	}
	m.mu.Lock()
	chal, ok := m.challenges[strings.ToLower(wallet)]
	if ok && time.Now().After(chal.ExpiresAt) {
		delete(m.challenges, strings.ToLower(wallet))
		ok = false
	}
	m.mu.Unlock()
	if !ok {
		return User{}, errors.New("challenge missing or expired")
	}
	if err := VerifyNeoSignature(wallet, signature, chal.Nonce, pubKey); err != nil {
		return User{}, err
	}
	user := User{Username: wallet, Role: "user"}
	m.mu.Lock()
	delete(m.challenges, strings.ToLower(wallet))
	m.mu.Unlock()
	return user, nil
}

func deriveNonce(wallet string, secret []byte, exp time.Time) string {
	// Simple HKDF-based nonce from wallet + expiry + secret
	info := []byte(fmt.Sprintf("%s|%d", wallet, exp.Unix()))
	h := hkdf.New(sha3.New256, secret, []byte(wallet), info)
	buf := make([]byte, 16)
	_, _ = h.Read(buf)
	return fmt.Sprintf("%x", buf)
}
