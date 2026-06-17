package tokens

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codeatlas/apps/auth-service/internal/users"
)

type Claims struct {
	Subject  int64  `json:"sub"`
	GitHubID int64  `json:"github_id"`
	Username string `json:"username"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (m *Manager) Issue(user users.User) (string, error) {
	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}

	now := time.Now().UTC()
	claimsJSON, err := json.Marshal(Claims{
		Subject:  user.ID,
		GitHubID: user.GitHubID,
		Username: user.Username,
		Expiry:   now.Add(m.ttl).Unix(),
		IssuedAt: now.Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}

	encodedHeader := encodeSegment(headerJSON)
	encodedClaims := encodeSegment(claimsJSON)
	unsignedToken := encodedHeader + "." + encodedClaims
	signature := m.sign(unsignedToken)

	return unsignedToken + "." + signature, nil
}

func (m *Manager) Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, fmt.Errorf("invalid token format")
	}

	unsignedToken := parts[0] + "." + parts[1]
	expectedSignature := m.sign(unsignedToken)
	if !hmac.Equal([]byte(expectedSignature), []byte(parts[2])) {
		return Claims{}, fmt.Errorf("invalid token signature")
	}

	payload, err := decodeSegment(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("decode token payload: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, fmt.Errorf("unmarshal token claims: %w", err)
	}

	if time.Now().UTC().Unix() > claims.Expiry {
		return Claims{}, fmt.Errorf("token expired")
	}

	return claims, nil
}

func (m *Manager) sign(data string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(data))
	return encodeSegment(mac.Sum(nil))
}

func encodeSegment(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeSegment(data string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(data)
}
