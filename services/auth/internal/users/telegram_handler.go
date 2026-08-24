package users

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"movie-platform/auth/internal/transport/http/response"
)

type TelegramHandler struct {
	service       *Service
	serviceSecret string
}

func NewTelegramHandler(
	service *Service,
	serviceSecret string,
) *TelegramHandler {
	return &TelegramHandler{
		service:       service,
		serviceSecret: serviceSecret,
	}
}

type telegramLinkCodeResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type telegramLinkRequest struct {
	Code           string `json:"code"`
	TelegramUserID int64  `json:"telegram_user_id"`
}

type telegramTokenRequest struct {
	TelegramUserID int64 `json:"telegram_user_id"`
}

type telegramTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func (
	handler *TelegramHandler,
) CreateLinkCode(
	w http.ResponseWriter,
	r *http.Request,
) {
	token, ok :=
		readBearerToken(r)

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"invalid authorization header",
		)
		return
	}

	result, err :=
		handler.service.
			CreateTelegramLinkCode(
				r.Context(),
				token,
			)

	if err != nil {
		if errors.Is(
			err,
			ErrInvalidAccessToken,
		) {
			response.Error(
				w,
				http.StatusUnauthorized,
				"unauthorized",
				"invalid access token",
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

	response.JSON(
		w,
		http.StatusCreated,
		telegramLinkCodeResponse{
			Code: result.Code,

			ExpiresAt: result.ExpiresAt,
		},
	)
}

func (
	handler *TelegramHandler,
) Link(
	w http.ResponseWriter,
	r *http.Request,
) {
	if !handler.authorizedService(r) {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"invalid service token",
		)
		return
	}

	var request telegramLinkRequest

	decoder :=
		json.NewDecoder(
			http.MaxBytesReader(
				w,
				r.Body,
				4096,
			),
		)

	decoder.DisallowUnknownFields()

	if err :=
		decoder.Decode(
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

	err :=
		handler.service.LinkTelegram(
			r.Context(),
			request.Code,
			request.TelegramUserID,
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			ErrTelegramLinkCodeNotFound,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid_link_code",
				"link code is invalid or expired",
			)

		case errors.Is(
			err,
			ErrTelegramAccountAlreadyLinked,
		):
			response.Error(
				w,
				http.StatusConflict,
				"telegram_already_linked",
				"telegram account is already linked",
			)

		case errors.Is(
			err,
			ErrMovieTrackerAccountAlreadyLinked,
		):
			response.Error(
				w,
				http.StatusConflict,
				"account_already_linked",
				"MovieTracker account is already linked",
			)

		case errors.Is(
			err,
			ErrInvalidTelegramUserID,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid_telegram_user_id",
				"invalid telegram user id",
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

	w.WriteHeader(
		http.StatusNoContent,
	)
}

func (
	handler *TelegramHandler,
) Token(
	w http.ResponseWriter,
	r *http.Request,
) {
	if !handler.authorizedService(r) {
		response.Error(
			w,
			http.StatusUnauthorized,
			"unauthorized",
			"invalid service token",
		)
		return
	}

	var request telegramTokenRequest

	decoder :=
		json.NewDecoder(
			http.MaxBytesReader(
				w,
				r.Body,
				4096,
			),
		)

	decoder.DisallowUnknownFields()

	if err :=
		decoder.Decode(
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

	accessToken, err :=
		handler.service.
			CreateTokenForTelegram(
				r.Context(),
				request.TelegramUserID,
			)

	if err != nil {
		switch {
		case errors.Is(
			err,
			ErrTelegramUserNotLinked,
		):
			response.Error(
				w,
				http.StatusNotFound,
				"telegram_not_linked",
				"telegram account is not linked",
			)

		case errors.Is(
			err,
			ErrInvalidTelegramUserID,
		):
			response.Error(
				w,
				http.StatusBadRequest,
				"invalid_telegram_user_id",
				"invalid telegram user id",
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
		http.StatusOK,
		telegramTokenResponse{
			AccessToken: accessToken,

			TokenType: "Bearer",
		},
	)
}

func (
	handler *TelegramHandler,
) authorizedService(
	r *http.Request,
) bool {
	received :=
		r.Header.Get(
			"X-Service-Token",
		)

	if received == "" ||
		handler.serviceSecret == "" {

		return false
	}

	expectedHash :=
		sha256.Sum256(
			[]byte(
				handler.serviceSecret,
			),
		)

	receivedHash :=
		sha256.Sum256(
			[]byte(received),
		)

	return subtle.ConstantTimeCompare(
		expectedHash[:],
		receivedHash[:],
	) == 1
}

func readBearerToken(
	r *http.Request,
) (string, bool) {
	parts :=
		strings.Fields(
			r.Header.Get(
				"Authorization",
			),
		)

	if len(parts) != 2 {
		return "", false
	}

	if !strings.EqualFold(
		parts[0],
		"Bearer",
	) {
		return "", false
	}

	if parts[1] == "" {
		return "", false
	}

	return parts[1], true
}
