package auth

import (
	"testing"

	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/models"
)

func TestCreateSuperuser(t *testing.T) {
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test-auth",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	tests := []struct {
		name string
	}{
		{name: "FirstCallCreates"},
		{name: "SecondCallIsIdempotent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateSuperuser(mongo, "test@conure.io", ""); err != nil {
				t.Errorf("CreateSuperuser() error = %v, want nil", err)
			}
		})
	}
}

func TestResetSuperuserPassword(t *testing.T) {
	config := &apiConfig.Config{
		JWTSecret:   "test-secret",
		MongoDBURI:  "mongodb://localhost:27017",
		MongoDBName: "conure-test-auth",
	}
	mongo, _ := database.ConnectToMongoDB(config.MongoDBURI, config.MongoDBName)
	defer cleanUpDB(mongo)

	tests := []struct {
		name      string
		seedUser  bool
		wantError bool
	}{
		{name: "MissingUserReturnsError", seedUser: false, wantError: true},
		{name: "ExistingUserGetsNewPassword", seedUser: true, wantError: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := &models.User{}
			after := &models.User{}
			if tt.seedUser {
				if err := CreateSuperuser(mongo, "test@conure.io", ""); err != nil {
					t.Fatalf("seed CreateSuperuser() error = %v", err)
				}
				if err := before.GetByEmail(mongo, "test@conure.io"); err != nil {
					t.Fatalf("seed GetByEmail() error = %v", err)
				}
			}
			err := ResetSuperuserPassword(mongo, "test@conure.io", "")
			if (err != nil) != tt.wantError {
				t.Errorf("ResetSuperuserPassword() error = %v, wantError = %v", err, tt.wantError)
			}
			if !tt.wantError {
				if err := after.GetByEmail(mongo, "test@conure.io"); err != nil {
					t.Fatalf("post GetByEmail() error = %v", err)
				}
				if after.Password == before.Password {
					t.Errorf("ResetSuperuserPassword() password unchanged, want new password")
				}
			}
		})
	}
}
