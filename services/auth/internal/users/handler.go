package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"movie-platform/auth/internal/transport/http/response"
)

type Handler struct {
	service *Service
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	User *User `json:"user"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// Register godoc
// @Summary Register user
// @Description Creates new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param request body registerRequest true "Register data"
// @Success 201 {object} registerResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /auth/register [post]
func (handler *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var request registerRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json", "invalid json")
		return
	}

	user, err := handler.service.Register(r.Context(), RegisterParams{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		if errors.Is(err, ErrEmailRequired) {
			response.Error(w, http.StatusBadRequest, "email_required", "email is required")
			return
		}

		if errors.Is(err, ErrInvalidEmail) {
			response.Error(w, http.StatusBadRequest, "invalid_email", "email is invalid")
			return
		}

		if errors.Is(err, ErrPasswordTooShort) {
			response.Error(w, http.StatusBadRequest, "password_too_short", "password must contain at least 8 characters")
			return
		}

		if errors.Is(err, ErrUserAlreadyExists) {
			response.Error(w, http.StatusConflict, "user_already_exists", "user already exists")
			return
		}

		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	response.JSON(w, http.StatusCreated, registerResponse{
		User: user,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	User        *User  `json:"user"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// Login godoc
// @Summary Login user
// @Description Authenticates user and returns access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body loginRequest true "Login data"
// @Success 200 {object} loginResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /auth/login [post]
func (handler *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_json", "invalid json")
		return
	}

	result, err := handler.service.Login(r.Context(), LoginParams{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		if errors.Is(err, ErrEmailRequired) {
			response.Error(w, http.StatusBadRequest, "email_required", "email is required")
			return
		}

		if errors.Is(err, ErrInvalidEmail) {
			response.Error(w, http.StatusBadRequest, "invalid_email", "email is invalid")
			return
		}

		if errors.Is(err, ErrInvalidCredentials) {
			response.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}

		response.Error(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	response.JSON(w, http.StatusOK, loginResponse{
		User:        result.User,
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
	})
}

// Logout godoc
// @Summary Logout user
// @Description Client-side logout. Client should delete access token after this request.
// @Tags auth
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 401 {object} response.ErrorResponse
// @Router /auth/logout [post]
func (handler *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "authorization header is required")
		return
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "invalid authorization header")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
