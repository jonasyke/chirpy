package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "mySuperSecret123!",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false,
		},
		{
			name:     "very long password",
			password: strings.Repeat("a", 1000),
			wantErr:  false,
		},
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
		{
			name:      "correct password",
			password:  plainPassword,
			hash:      validHash,
			wantMatch: true,
			wantErr:   false,
		},
		{
			name:      "wrong password",
			password:  "wrongPassword123",
			hash:      validHash,
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "empty password",
			password:  "",
			hash:      validHash,
			wantMatch: false,
			wantErr:   false,
		},
		{
			name:      "invalid hash format",
			password:  plainPassword,
			hash:      "not-a-valid-hash",
			wantMatch: false,
			wantErr:   true,
		},
		{
			name:      "empty hash",
			password:  plainPassword,
			hash:      "",
			wantMatch: false,
			wantErr:   true,
		},
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
