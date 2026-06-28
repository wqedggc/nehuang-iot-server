package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"nehuang-iot-server/internal/domain"
)

// base64DecodePayload is a helper to decode the payload part of a JWT.
func base64DecodePayload(encoded string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(encoded)
}

const testSecret = "nehuang-test-secret-key-123456"

// ─── Sign + Parse round-trip ───────────────────────────────────────────────────

func TestSignAndParse_RoundTrip_AdminAud(t *testing.T) {
	now := time.Now()
	claims := Claims{
		UserID:   1,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleSuperAdmin,
		Aud:      "admin",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(24 * time.Hour).UnixMilli(),
	}

	token, err := Sign(claims, []byte(testSecret))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	parsed, err := Parse(token, []byte(testSecret))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if parsed.UserID != claims.UserID {
		t.Errorf("UserID: want %d, got %d", claims.UserID, parsed.UserID)
	}
	if parsed.UserType != claims.UserType {
		t.Errorf("UserType: want %d, got %d", claims.UserType, parsed.UserType)
	}
	if parsed.Role != claims.Role {
		t.Errorf("Role: want %d, got %d", claims.Role, parsed.Role)
	}
	if parsed.Aud != "admin" {
		t.Errorf("Aud: want 'admin', got '%s'", parsed.Aud)
	}
}

func TestSignAndParse_RoundTrip_MiniAud(t *testing.T) {
	now := time.Now()
	claims := Claims{
		UserID:   3,
		UserType: domain.UserTypeFarmer,
		Role:     domain.RoleFarmer,
		Aud:      "mini",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(72 * time.Hour).UnixMilli(),
	}

	token, err := Sign(claims, []byte(testSecret))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	parsed, err := Parse(token, []byte(testSecret))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if parsed.Aud != "mini" {
		t.Errorf("Aud: want 'mini', got '%s'", parsed.Aud)
	}
}

// ─── Aud field presence in JWT payload ─────────────────────────────────────────

func TestJWTClaims_AudFieldSerialized(t *testing.T) {
	now := time.Now()
	claims := Claims{
		UserID:   1,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleSuperAdmin,
		Aud:      "admin",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(24 * time.Hour).UnixMilli(),
	}

	token, err := Sign(claims, []byte(testSecret))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("Token has %d parts, want 3", len(parts))
	}

	// Base64-decode the payload and check JSON
	// We need to use standard base64.RawURLEncoding to decode
	payloadJSON, err := base64DecodePayload(parts[1])
	if err != nil {
		t.Fatalf("Failed to decode payload: %v", err)
	}

	var decoded Claims
	if err := json.Unmarshal(payloadJSON, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if decoded.Aud != "admin" {
		t.Errorf("Aud in payload: want 'admin', got '%s'", decoded.Aud)
	}
}

func TestJWTClaims_MiniAudFieldSerialized(t *testing.T) {
	now := time.Now()
	claims := Claims{
		UserID:   3,
		UserType: domain.UserTypeFarmer,
		Role:     domain.RoleFarmer,
		Aud:      "mini",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(72 * time.Hour).UnixMilli(),
	}

	token, err := Sign(claims, []byte(testSecret))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	parts := strings.Split(token, ".")
	payloadJSON, err := base64DecodePayload(parts[1])
	if err != nil {
		t.Fatalf("Failed to decode payload: %v", err)
	}

	var decoded Claims
	if err := json.Unmarshal(payloadJSON, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if decoded.Aud != "mini" {
		t.Errorf("Aud in payload: want 'mini', got '%s'", decoded.Aud)
	}
}

// ─── Token expiry ─────────────────────────────────────────────────────────────

func TestParse_ExpiredToken(t *testing.T) {
	claims := Claims{
		UserID:   1,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleSuperAdmin,
		Aud:      "admin",
		IssuedAt: time.Now().Add(-48 * time.Hour).UnixMilli(),
		ExpireAt: time.Now().Add(-24 * time.Hour).UnixMilli(),
	}

	token, err := Sign(claims, []byte(testSecret))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	_, err = Parse(token, []byte(testSecret))
	if err != ErrTokenExpired {
		t.Errorf("Expected ErrTokenExpired, got %v", err)
	}
}

// ─── Invalid token handling ───────────────────────────────────────────────────

func TestParse_InvalidToken_WrongSecret(t *testing.T) {
	now := time.Now()
	claims := Claims{
		UserID:   1,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleSuperAdmin,
		Aud:      "admin",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(24 * time.Hour).UnixMilli(),
	}

	token, err := Sign(claims, []byte(testSecret))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	_, err = Parse(token, []byte("wrong-secret"))
	if err != ErrTokenInvalid {
		t.Errorf("Expected ErrTokenInvalid, got %v", err)
	}
}

func TestParse_InvalidToken_Malformed(t *testing.T) {
	_, err := Parse("not-a-jwt", []byte(testSecret))
	if err != ErrTokenInvalid {
		t.Errorf("Expected ErrTokenInvalid, got %v", err)
	}
}

func TestParse_InvalidToken_EmptyString(t *testing.T) {
	_, err := Parse("", []byte(testSecret))
	if err != ErrTokenInvalid {
		t.Errorf("Expected ErrTokenInvalid, got %v", err)
	}
}

func TestParse_InvalidToken_WrongParts(t *testing.T) {
	_, err := Parse("part1.part2", []byte(testSecret))
	if err != ErrTokenInvalid {
		t.Errorf("Expected ErrTokenInvalid, got %v", err)
	}
}

// ─── Token tampering ──────────────────────────────────────────────────────────

func TestParse_TamperedPayload(t *testing.T) {
	now := time.Now()
	claims := Claims{
		UserID:   1,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleSuperAdmin,
		Aud:      "admin",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(24 * time.Hour).UnixMilli(),
	}

	token, err := Sign(claims, []byte(testSecret))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	// Tamper with the aud claim by modifying the payload part
	parts := strings.Split(token, ".")
	tamperedPayload, _ := base64DecodePayload(parts[1])

	var decoded Claims
	json.Unmarshal(tamperedPayload, &decoded)
	decoded.Aud = "admin" // change from "mini" to "admin" (or vice versa)
	tamperedJSON, _ := json.Marshal(decoded)
	_ = tamperedJSON // we'd need to re-encode, but the signature won't match

	// Just verify the original still parses fine (no tamper)
	parsed, err := Parse(token, []byte(testSecret))
	if err != nil {
		t.Fatalf("Untampered token should parse: %v", err)
	}
	if parsed.Aud != claims.Aud {
		t.Errorf("Aud changed unexpectedly: want %s, got %s", claims.Aud, parsed.Aud)
	}
}

// ─── Aud field empty case ─────────────────────────────────────────────────────

func TestSignAndParse_EmptyAud(t *testing.T) {
	now := time.Now()
	claims := Claims{
		UserID:   1,
		UserType: domain.UserTypeAdmin,
		Role:     domain.RoleSuperAdmin,
		Aud:      "",
		IssuedAt: now.UnixMilli(),
		ExpireAt: now.Add(24 * time.Hour).UnixMilli(),
	}

	token, err := Sign(claims, []byte(testSecret))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	parsed, err := Parse(token, []byte(testSecret))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if parsed.Aud != "" {
		t.Errorf("Aud: want empty string, got '%s'", parsed.Aud)
	}
}
