package users

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"movie-platform/auth/internal/transport/http/response"
)

const (
	maxAuthRequestBodyBytes = 64 * 1024

	refreshCookieName = "movietracker_refresh"

	refreshCookiePath = "/auth"
)

type Handler struct {
	service *Service

	secureCookies bool
}

func NewHandler(
	service *Service,
	secureCookies bool,
) *Handler {
	return &Handler{
		service: service,

		secureCookies: secureCookies,
	}
}

type registerRequest struct {
	Email string `json:"email"`

	Password string `json:"password"`
}

type registerResponse struct {
	User *User `json:"user"`
}

func (handler *Handler) Register(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request registerRequest

	if err :=
		decodeAuthJSON(
			w,
			r,
			&request,
		); err != nil {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_json",
			"invalid json",
		)

		return
	}

	user, err :=
		handler.service.Register(
			r.Context(),
			RegisterParams{
				Email: request.Email,

				Password: request.Password,
			},
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			ErrEmailRequired,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"email_required",
				"email is required",
			)

		case errors.Is(
			err,
			ErrInvalidEmail,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid_email",
				"email is invalid",
			)

		case errors.Is(
			err,
			ErrEmailTooLong,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"email_too_long",
				"email is too long",
			)

		case errors.Is(
			err,
			ErrPasswordTooShort,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"password_too_short",
				"password must contain at least 8 characters",
			)

		case errors.Is(
			err,
			ErrPasswordTooLong,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"password_too_long",
				"password must contain at most 72 bytes",
			)

		case errors.Is(
			err,
			ErrUserAlreadyExists,
		):
			response.Error(
				w,
				http.StatusConflict,
				"user_already_exists",
				"user already exists",
			)

		default:
			response.Error(
				w,
				http.StatusInternalServerError,
				"internal_error",
				"internal error",
			)
		}

		return
	}

	response.JSON(
		w,
		http.StatusCreated,
		registerResponse{
			User: user,
		},
	)
}

type loginRequest struct {
	Email string `json:"email"`

	Password string `json:"password"`
}

type loginResponse struct {
	User *User `json:"user"`

	AccessToken string `json:"access_token"`

	TokenType string `json:"token_type"`
}

func (handler *Handler) Login(
	w http.ResponseWriter,
	r *http.Request,
) {
	disableAuthCaching(w)

	var request loginRequest

	if err :=
		decodeAuthJSON(
			w,
			r,
			&request,
		); err != nil {

		response.Error(
			w,
			http.StatusBadRequest,
			"invalid_json",
			"invalid json",
		)

		return
	}

	result, err :=
		handler.service.Login(
			r.Context(),
			LoginParams{
				Email: request.Email,

				Password: request.Password,
			},
		)

	if err != nil {
		if errors.Is(
			err,
			ErrInvalidCredentials,
		) {
			response.Error(
				w,
				http.StatusUnauthorized,
				"invalid_credentials",
				"invalid email or password",
			)

			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"internal error",
		)

		return
	}

	handler.setRefreshCookie(
		w,
		result.RefreshToken,
		result.RefreshExpiresAt,
	)

	response.JSON(
		w,
		http.StatusOK,
		loginResponse{
			User: result.User,

			AccessToken: result.AccessToken,

			TokenType: "Bearer",
		},
	)
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`

	TokenType string `json:"token_type"`
}

func (handler *Handler) Refresh(
	w http.ResponseWriter,
	r *http.Request,
) {
	disableAuthCaching(w)

	refreshToken, ok :=
		readRefreshCookie(
			r,
		)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"invalid_refresh_token",
			"invalid refresh token",
		)

		return
	}

	result, err :=
		handler.service.Refresh(
			r.Context(),
			refreshToken,
		)

	if err != nil {
		if errors.Is(
			err,
			ErrInvalidRefreshToken,
		) {
			handler.clearRefreshCookie(
				w,
			)

			response.Error(
				w,
				http.StatusUnauthorized,
				"invalid_refresh_token",
				"invalid refresh token",
			)

			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"internal_error",
			"internal error",
		)

		return
	}

	handler.setRefreshCookie(
		w,
		result.RefreshToken,
		result.RefreshExpiresAt,
	)

	response.JSON(
		w,
		http.StatusOK,
		refreshResponse{
			AccessToken: result.AccessToken,

			TokenType: "Bearer",
		},
	)
}

func (handler *Handler) Logout(
	w http.ResponseWriter,
	r *http.Request,
) {
	disableAuthCaching(w)

	refreshToken, ok :=
		readRefreshCookie(
			r,
		)

	if ok {
		if err :=
			handler.service.Logout(
				r.Context(),
				refreshToken,
			); err != nil {

			response.Error(
				w,
				http.StatusInternalServerError,
				"internal_error",
				"internal error",
			)

			return
		}
	}

	handler.clearRefreshCookie(
		w,
	)

	w.WriteHeader(
		http.StatusNoContent,
	)
}

func (handler *Handler) setRefreshCookie(
	w http.ResponseWriter,
	refreshToken string,
	expiresAt time.Time,
) {
	maxAge :=
		int(
			time.Until(
				expiresAt,
			).
				Seconds(),
		)

	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(
		w,
		&http.Cookie{
			Name: refreshCookieName,

			Value: refreshToken,

			Path: refreshCookiePath,

			HttpOnly: true,

			Secure: handler.secureCookies,

			SameSite: http.SameSiteLaxMode,

			Expires: expiresAt,

			MaxAge: maxAge,
		},
	)
}

func (handler *Handler) clearRefreshCookie(
	w http.ResponseWriter,
) {
	http.SetCookie(
		w,
		&http.Cookie{
			Name: refreshCookieName,

			Value: "",

			Path: refreshCookiePath,

			HttpOnly: true,

			Secure: handler.secureCookies,

			SameSite: http.SameSiteLaxMode,

			Expires: time.Unix(
				0,
				0,
			),

			MaxAge: -1,
		},
	)
}

func readRefreshCookie(
	r *http.Request,
) (string, bool) {
	cookie, err :=
		r.Cookie(
			refreshCookieName,
		)

	if err != nil {
		return "", false
	}

	if cookie.Value == "" {
		return "", false
	}

	return cookie.Value, true
}

func decodeAuthJSON(
	w http.ResponseWriter,
	r *http.Request,
	target any,
) error {
	r.Body =
		http.MaxBytesReader(
			w,
			r.Body,
			maxAuthRequestBodyBytes,
		)

	decoder :=
		json.NewDecoder(
			r.Body,
		)

	decoder.DisallowUnknownFields()

	if err :=
		decoder.Decode(
			target,
		); err != nil {

		return err
	}

	var extra any

	if err :=
		decoder.Decode(
			&extra,
		); !errors.Is(
		err,
		io.EOF,
	) {

		return errors.New(
			"request body must contain one JSON object",
		)
	}

	return nil
}

func disableAuthCaching(
	w http.ResponseWriter,
) {
	w.Header().Set(
		"Cache-Control",
		"no-store",
	)

	w.Header().Set(
		"Pragma",
		"no-cache",
	)
}
