package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateFromPassword(t *testing.T) {
	type args struct {
		password string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "TestGenerateFromPassword",
			args:    args{password: "password"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashed, err := GenerateFromPassword(tt.args.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateFromPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// hashed password always contains the same prefix, check for it ($argon2)
			if hashed[:7] != "$argon2" {
				t.Errorf("GenerateFromPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

		})
	}
}

func TestGenerateRandomBytes(t *testing.T) {
	type args struct {
		n uint32
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "TestGenerateRandomBytes",
			args:    args{n: 16},
			wantErr: false,
		}, {
			name:    "TestGenerateRandomBytes",
			args:    args{n: 0},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateRandomBytes(tt.args.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRandomBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestComparePasswordAndHash(t *testing.T) {
	type args struct {
		password    string
		encodedHash string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name:    "TestComparePasswordAndHash",
			args:    args{password: "password", encodedHash: "$argon2id$v=19$m=65536,t=3,p=2$GKI4lZoCLqHSGbiMWfKkOg$6EUwhZn01UsOb4lIQt/EzEh7ytZw/Wv3i1PaDisMdZY"},
			want:    true,
			wantErr: false,
		}, {
			name:    "TestComparePasswordAndHash",
			args:    args{password: "password", encodedHash: "$argon2id$v=20$m=65536,t=3,p=2$GKI4lZoCLqHSGbiMWfKkOg$6EUwhZn01UsOb4lIQt/EzEh7ytZw/Wv3i1PaDisMdZY"},
			want:    false,
			wantErr: true,
		}, { // Test with wrong password
			name:    "TestComparePasswordAndHash",
			args:    args{password: "wrongpassword", encodedHash: "$argon2id$v=19$m=65536,t=3,p=2$Zm9v$Zm9v"},
			want:    false,
			wantErr: false,
		}, { // Test with wrong hash
			name:    "TestComparePasswordAndHash",
			args:    args{password: "password", encodedHash: "$argon2id$v=19$m=65536,t=3,p=2$Zm9v$Zm9v"},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComparePasswordAndHash(tt.args.password, tt.args.encodedHash)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComparePasswordAndHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ComparePasswordAndHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	type args struct {
		length int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "TestGenerateRandomPassword",
			args:    args{length: 16},
			wantErr: false,
		}, {
			name:    "TestGenerateRandomPassword",
			args:    args{length: 0},
			wantErr: false,
		}, {
			name:    "TestGenerateRandomPassword",
			args:    args{length: -1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantErr {
					t.Errorf("SequenceInt() recover = %v, wantPanic = %v", r, tt.wantErr)
				}
			}()
			got := GenerateRandomPassword(tt.args.length)
			if len(got) != int(tt.args.length) {
				t.Errorf("GenerateRandomPassword() = %v, want %v", len(got), tt.args.length)
			}
		})
	}
}

func TestGenerateToken(t *testing.T) {
	testJWTSecretKey := "secret"
	testPayload := JWTData{
		Email:  "test@example.com",
		Client: "fake-client",
	}
	ttl := 1 * time.Hour
	tokenString, err := GenerateToken(ttl, testPayload, testJWTSecretKey)

	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(testJWTSecretKey), nil
	})

	require.NoError(t, err)
	require.True(t, token.Valid)

	if claims, ok := token.Claims.(*JWTClaims); ok {
		assert.Equal(t, testPayload.Email, claims.Subject)
		assert.WithinDuration(t, time.Now().UTC().Add(ttl), time.Unix(claims.ExpiresAt, 0), 5*time.Second)
	} else {
		t.FailNow()
	}
}

func TestGenerateTokenError(t *testing.T) {
	testPayload := JWTData{
		Email:  "test@example.com",
		Client: "fake-client",
	}

	_, err := GenerateToken(1*time.Hour, testPayload, "")
	require.Error(t, err)
}

func TestValidateToken(t *testing.T) {
	testJWTSecret := "secret"
	testPayload := JWTData{
		Email:  "test@example.com",
		Client: "fake-client",
	}
	ttl := 1 * time.Hour
	tokenString, err := GenerateToken(ttl, testPayload, testJWTSecret)

	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	claims, err := ValidateToken(tokenString, testJWTSecret)
	require.NoError(t, err)
	require.Equal(t, testPayload.Email, claims.Subject)

	// Test with expired token
	tokenString, err = GenerateToken(-1*time.Hour, testPayload, testJWTSecret)
	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	_, err = ValidateToken(tokenString, testJWTSecret)
	require.Error(t, err)

	// Test with invalid token
	_, err = ValidateToken("invalid-token", testJWTSecret)
	require.Error(t, err)

	// Test with invalid secret
	_, err = ValidateToken(tokenString, "invalid-secret")
	require.Error(t, err)
}
