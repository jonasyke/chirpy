package auth

import (
	"strings"
	"testing"
	"time"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "valid password", password: "mySuperSecret123!", wantErr: false},
		{name: "empty password", password: "", wantErr: false},
		{name: "very long password", password: strings.Repeat("a", 1000), wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)

			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && hash == "" {
				t.Error("HashPassword() returned empty hash but no error")
			}

			if !tt.wantErr && !strings.HasPrefix(hash, "$argon2id$v=19$") {
				t.Errorf("HashPassword() returned invalid format: %s", hash)
			}
		})
	}
}

func TestCheckPasswordHash(t *testing.T) {
	plainPassword := "correctHorseBatteryStaple"
	validHash, err := HashPassword(plainPassword)
	if err != nil {
		t.Fatalf("Failed to create test hash: %v", err)
	}

	tests := []struct {
		name      string
		password  string
		hash      string
		wantMatch bool
		wantErr   bool
	}{
		{name: "correct password", password: plainPassword, hash: validHash, wantMatch: true, wantErr: false},
		{name: "wrong password", password: "wrongPassword123", hash: validHash, wantMatch: false, wantErr: false},
		{name: "empty password", password: "", hash: validHash, wantMatch: false, wantErr: false},
		{name: "invalid hash format", password: plainPassword, hash: "not-a-valid-hash", wantMatch: false, wantErr: true},
		{name: "empty hash", password: plainPassword, hash: "", wantMatch: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := CheckPasswordHash(tt.password, tt.hash)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPasswordHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if match != tt.wantMatch {
				t.Errorf("CheckPasswordHash() match = %v, want %v", match, tt.wantMatch)
			}
		})
	}
}

func TestHashPassword_ProducesDifferentHashes(t *testing.T) {
	password := "samePassword123!"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	if hash1 == hash2 {
		t.Error("HashPassword() produced identical hashes for the same password (salt should be random)")
	}
}

const testSecret = "super-secret-key-for-testing-only"

func TestMakeJWT(t *testing.T) {
	userID := uuid.New()
	expiresIn := 24 * time.Hour

	tokenStr, err := MakeJWT(userID, testSecret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT failed: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("expected non-empty token string")
	}

	parsedToken, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(testSecret), nil
	})
	if err != nil || !parsedToken.Valid {
		t.Fatalf("failed to parse generated token: %v", err)
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims type assertion failed, got %T", parsedToken.Claims)
	}

	if claims["iss"] != "chirpy-access" {
		t.Errorf("expected Issuer 'chirpy-access', got %v", claims["iss"])
	}
	if claims["sub"] != userID.String() {
		t.Errorf("expected Subject %s, got %v", userID.String(), claims["sub"])
	}

	// Check expiration
	if exp, ok := claims["exp"].(float64); !ok || time.Unix(int64(exp), 0).Before(time.Now()) {
		t.Error("token should have a valid future ExpiresAt")
	}
}

func TestMakeJWT_EmptySecret(t *testing.T) {
	userID := uuid.New()

	tokenStr, err := MakeJWT(userID, "", 1*time.Hour)
	if err != nil {
		t.Errorf("MakeJWT with empty secret returned unexpected error: %v", err)
	}
	if tokenStr == "" {
		t.Error("expected non-empty token even with empty secret")
	}
}

func TestValidateJWT(t *testing.T) {
	userID := uuid.New()
	expiresIn := 1 * time.Hour

	validToken, err := MakeJWT(userID, testSecret, expiresIn)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		tokenString string
		secret      string
		wantErr     bool
	}{
		{
			name:        "valid token",
			tokenString: validToken,
			secret:      testSecret,
			wantErr:     false,
		},
		{
			name:        "expired token",
			tokenString: func() string {
				tok, _ := MakeJWT(userID, testSecret, -1*time.Second)
				return tok
			}(),
			secret:  testSecret,
			wantErr: true,
		},
		{
			name:        "wrong secret",
			tokenString: validToken,
			secret:      "wrong-secret",
			wantErr:     true,
		},
		{
			name:        "malformed token",
			tokenString: "not.a.valid.token",
			secret:      testSecret,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateJWT(tt.tokenString, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJWT() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	tests := []struct {
		name        string
		authHeader  string
		wantToken   string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid Bearer token - standard case",
			authHeader: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantToken:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantErr:    false,
		},
		{
			name:       "invalid Bearer token - lowercase bearer",
			authHeader: "bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantToken:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantErr:    true,
		},
		{
			name:       "invalid Bearer token - mixed case",
			authHeader: "BeArEr eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantToken:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantErr:    true,
		},
		{
			name:       "valid with extra whitespace",
			authHeader: "  Bearer   eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...  ",
			wantToken:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantErr:    true,
		},
		{
			name:        "missing Authorization header",
			authHeader:  "",
			wantToken:   "",
			wantErr:     true,
			errContains: "authorization header not found",
		},
		{
			name:        "malformed - only one part",
			authHeader:  "Bearer",
			wantToken:   "",
			wantErr:     true,
			errContains: "malformed authorization header",
		},
		{
			name:        "malformed - three parts",
			authHeader:  "Bearer token extra",
			wantToken:   "",
			wantErr:     true,
			errContains: "malformed authorization header",
		},
		{
			name:        "wrong scheme - Basic auth",
			authHeader:  "Basic dXNlcjpwYXNz",
			wantToken:   "",
			wantErr:     true,
			errContains: "malformed authorization header",
		},
		{
			name:        "wrong scheme - no space",
			authHeader:  "BearereyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantToken:   "",
			wantErr:     true,
			errContains: "malformed authorization header",
		},
		{
			name:        "empty token",
			authHeader:  "Bearer ",
			wantToken:   "",
			wantErr:     true,
			errContains: "malformed authorization header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.authHeader != "" {
				headers.Set("Authorization", tt.authHeader)
			}

			gotToken, err := GetBearerToken(headers)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if gotToken != tt.wantToken {
					t.Errorf("got token %q, want %q", gotToken, tt.wantToken)
				}
			}
		})
	}
}