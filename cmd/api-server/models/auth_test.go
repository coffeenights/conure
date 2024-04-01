package models

import "testing"

func TestValidateEmail(t *testing.T) {
	type args struct {
		email string
	}
	tests := []struct {
		name      string
		args      args
		wantError bool
	}{
		{
			name:      "TestValidateEmail",
			args:      args{email: "test@test.com"},
			wantError: false,
		}, {
			name:      "TestValidateEmail",
			args:      args{email: "test@test"},
			wantError: true,
		}, {
			name:      "TestValidateEmail",
			args:      args{email: "test"},
			wantError: true,
		}, {
			name:      "TestValidateEmail",
			args:      args{email: "test@"},
			wantError: true,
		}, {
			name:      "TestValidateEmail",
			args:      args{email: "test.com"},
			wantError: true,
		}, {
			name:      "TestValidateEmail",
			args:      args{email: "test@.com"},
			wantError: true,
		}, {
			name:      "TestValidateEmail",
			args:      args{email: "test@com"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.args.email)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateEmail() error = %v, wantError %v", err, tt.wantError)
				return
			}
		})
	}
}

func TestValidatePasswords(t *testing.T) {
	type args struct {
		password  string
		password2 string
	}
	tests := []struct {
		name      string
		args      args
		wantError bool
	}{
		{
			name:      "TestValidatePasswords",
			args:      args{password: "Password123+", password2: "Password123+"},
			wantError: false,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "password123", password2: "password123"},
			wantError: true,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "password123+", password2: "password123+"},
			wantError: true,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "PASSWORD", password2: "PASSWORD"},
			wantError: true,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "Password+", password2: "Password+"},
			wantError: true,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "password", password2: "password"},
			wantError: true,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "pass", password2: "pass"},
			wantError: true,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "password", password2: "password2"},
			wantError: true,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "password", password2: ""},
			wantError: true,
		}, {
			name:      "TestValidatePasswords",
			args:      args{password: "", password2: "password"},
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswords(tt.args.password, tt.args.password2)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidatePasswords() error = %v, wantError %v", err, tt.wantError)
				return
			}
		})
	}
}
