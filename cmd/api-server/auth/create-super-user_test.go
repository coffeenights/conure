package auth

import (
	"testing"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
)

func TestCreateSuperuser(t *testing.T) {
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	tests := []struct {
		name      string
		wantError bool
	}{
		{
			name:      "TestCreateSuperuser",
			wantError: false,
		},
		{
			name:      "TestCreateSuperuser",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantError {
					t.Errorf("SequenceInt() recover = %v, wantPanic = %v", r, tt.wantError)
				}
			}()
			CreateSuperuser(mongo)
		})
	}
}

func TestResetSuperuserPassword(t *testing.T) {
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	tests := []struct {
		name      string
		wantError bool
	}{
		{
			name:      "TestResetSuperuserPassword",
			wantError: true,
		},
		{
			name:      "TestResetSuperuserPassword",
			wantError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantError {
					t.Errorf("SequenceInt() recover = %v, wantPanic = %v", r, tt.wantError)
				}
			}()
			u := &User{}
			u2 := &User{}
			if !tt.wantError {
				CreateSuperuser(mongo)
				_ = u.GetByEmail(mongo, "admin@conure.io")
			}
			ResetSuperuserPassword(mongo)
			if !tt.wantError {
				_ = u2.GetByEmail(mongo, "admin@conure.io")
				if u2.Password == u.Password {
					t.Errorf("ResetSuperuserPassword() password = %v, want new password", u2.Password)
				}
			}
		})
	}
}
