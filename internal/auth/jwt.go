// Package auth provides JWT signing and verification utilities using HS256.
// No third-party JWT library is used; signing is done with crypto/hmac + crypto/sha256.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"nehuang-iot-server/internal/domain"
)

// Sentinel errors returned by Parse.
var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
)

// Claims holds the data embedded inside a JWT payload.
type Claims struct {
	UserID   uint64      `json:"user_id"`
	UserType domain.UserType `json:"user_type"`
	Role     domain.Role     `json:"role"`
	IssuedAt int64           `json:"iat"` // Unix milliseconds
	ExpireAt int64           `json:"exp"` // Unix milliseconds
}

// jwtHeader is the fixed base64url-encoded HS256 header.
// {"alg":"HS256","typ":"JWT"}
const jwtHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

// Sign encodes claims into a HS256 JWT string.
// The caller is responsible for setting Claims.IssuedAt and Claims.ExpireAt.
func Sign(claims Claims, secret []byte) (string, error) {
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	header := jwtHeader
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := header + "." + payload

	sig := computeHMAC([]byte(signingInput), secret)
	sigEncoded := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigEncoded, nil
}

// Parse validates the token's signature and expiry, then returns the Claims.
// Returns ErrTokenExpired if the token has expired, ErrTokenInvalid for any other failure.
func Parse(tokenStr string, secret []byte) (Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return Claims{}, ErrTokenInvalid
	}

	// Verify signature.
	signingInput := parts[0] + "." + parts[1]
	expectedSig := computeHMAC([]byte(signingInput), secret)
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}
	if !hmac.Equal(expectedSig, actualSig) {
		return Claims{}, ErrTokenInvalid
	}

	// Decode payload.
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return Claims{}, ErrTokenInvalid
	}

	// Check expiry (ExpireAt is Unix milliseconds).
	nowMS := time.Now().UnixMilli()
	if nowMS > claims.ExpireAt {
		return Claims{}, ErrTokenExpired
	}

	return claims, nil
}

// computeHMAC returns the HMAC-SHA256 digest of data using key.
func computeHMAC(data, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
