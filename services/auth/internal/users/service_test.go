package users

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRegisterValidation(t *testing.T) {
	service := NewService(
		nil,
		"test-jwt-secret",
		"test-issuer",
		"test-audience",
	)

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{
			name:     "empty email",
			email:    "",
			password: "password123",
			wantErr:  ErrEmailRequired,
		},
		{
			name:     "invalid email",
			email:    "not-an-email",
			password: "password123",
			wantErr:  ErrInvalidEmail,
		},
		{
			name:     "short password",
			email:    "test@example.com",
			password: "1234567",
			wantErr:  ErrPasswordTooShort,
		},
		{
			name:     "password too long",
			email:    "test@example.com",
			password: strings.Repeat("a", 73),
			wantErr:  ErrPasswordTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				_, err := service.Register(
					context.Background(),
					RegisterParams{
						Email:    tt.email,
						Password: tt.password,
					},
				)

				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.wantErr,
						err,
					)
				}
			},
		)
	}
}

func TestLoginRejectsInvalidInput(t *testing.T) {
	service := NewService(
		nil,
		"test-jwt-secret",
		"test-issuer",
		"test-audience",
	)

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{
			name:     "empty email",
			email:    "",
			password: "password123",
		},
		{
			name:     "password too long",
			email:    "test@example.com",
			password: strings.Repeat("a", 73),
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				_, err := service.Login(
					context.Background(),
					LoginParams{
						Email:    tt.email,
						Password: tt.password,
					},
				)

				if !errors.Is(err, ErrInvalidCredentials) {
					t.Fatalf(
						"expected ErrInvalidCredentials, got %v",
						err,
					)
				}
			},
		)
	}
}
