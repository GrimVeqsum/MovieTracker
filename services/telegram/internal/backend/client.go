package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrInvalidLinkCode = errors.New(
		"invalid or expired telegram link code",
	)

	ErrTelegramAlreadyLinked = errors.New(
		"telegram account already linked",
	)

	ErrAccountAlreadyLinked = errors.New(
		"movietracker account already linked",
	)

	ErrInvalidTelegramUserID = errors.New(
		"invalid telegram user id",
	)

	ErrUnauthorizedService = errors.New(
		"unauthorized service",
	)

	ErrTelegramNotLinked = errors.New(
		"telegram user is not linked",
	)

	ErrMovieNotFound = errors.New(
		"movie not found",
	)

	ErrLibraryUnauthorized = errors.New(
		"library authorization failed",
	)
)

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Movie struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	Title           string `json:"title"`
	NormalizedTitle string `json:"normalized_title"`

	ReleaseYear *int `json:"release_year"`

	ExternalID       *string `json:"external_id,omitempty"`
	MetadataProvider *string `json:"metadata_provider,omitempty"`
	OriginalTitle    *string `json:"original_title,omitempty"`
	Description      *string `json:"description,omitempty"`
	PosterURL        *string `json:"poster_url,omitempty"`
	RuntimeMinutes   *int    `json:"runtime_minutes,omitempty"`

	MetadataStatus string  `json:"metadata_status"`
	MetadataError  *string `json:"metadata_error,omitempty"`

	Genres []Genre `json:"genres,omitempty"`

	Status string  `json:"status"`
	Rating *int    `json:"rating"`
	Review *string `json:"review"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	WatchedAt *time.Time `json:"watched_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiErrorResponse struct {
	Error apiError `json:"error"`
}

type Client struct {
	authURL           string
	libraryURL        string
	authServiceSecret string
	httpClient        *http.Client
}

func NewClient(
	authURL string,
	libraryURL string,
	authServiceSecret string,
) *Client {
	return &Client{
		authURL: strings.TrimRight(
			authURL,
			"/",
		),

		libraryURL: strings.TrimRight(
			libraryURL,
			"/",
		),

		authServiceSecret: authServiceSecret,

		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (client *Client) AuthReady(
	ctx context.Context,
) error {
	return client.checkReady(
		ctx,
		client.authURL,
	)
}

func (client *Client) LibraryReady(
	ctx context.Context,
) error {
	return client.checkReady(
		ctx,
		client.libraryURL,
	)
}

func (client *Client) LinkTelegram(
	ctx context.Context,
	code string,
	telegramUserID int64,
) error {
	requestBody := struct {
		Code           string `json:"code"`
		TelegramUserID int64  `json:"telegram_user_id"`
	}{
		Code: code,

		TelegramUserID: telegramUserID,
	}

	data, err :=
		json.Marshal(
			requestBody,
		)

	if err != nil {
		return fmt.Errorf(
			"marshal telegram link request: %w",
			err,
		)
	}

	requestURL :=
		client.authURL +
			"/internal/telegram/link"

	req, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			requestURL,
			bytes.NewReader(data),
		)

	if err != nil {
		return fmt.Errorf(
			"create telegram link request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	req.Header.Set(
		"X-Service-Token",
		client.authServiceSecret,
	)

	resp, err :=
		client.httpClient.Do(req)

	if err != nil {
		return fmt.Errorf(
			"telegram link request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode ==
		http.StatusNoContent {

		return nil
	}

	apiErr, rawBody :=
		readAPIError(resp)

	switch apiErr.Code {
	case "invalid_link_code":
		return ErrInvalidLinkCode

	case "telegram_already_linked":
		return ErrTelegramAlreadyLinked

	case "account_already_linked":
		return ErrAccountAlreadyLinked

	case "invalid_telegram_user_id":
		return ErrInvalidTelegramUserID

	case "unauthorized":
		return ErrUnauthorizedService

	default:
		return fmt.Errorf(
			"auth-service returned status %d, code=%s, message=%s: %s",
			resp.StatusCode,
			apiErr.Code,
			apiErr.Message,
			rawBody,
		)
	}
}

func (client *Client) TelegramToken(
	ctx context.Context,
	telegramUserID int64,
) (string, error) {
	requestBody := struct {
		TelegramUserID int64 `json:"telegram_user_id"`
	}{
		TelegramUserID: telegramUserID,
	}

	data, err :=
		json.Marshal(
			requestBody,
		)

	if err != nil {
		return "", fmt.Errorf(
			"marshal telegram token request: %w",
			err,
		)
	}

	requestURL :=
		client.authURL +
			"/internal/telegram/token"

	req, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			requestURL,
			bytes.NewReader(data),
		)

	if err != nil {
		return "", fmt.Errorf(
			"create telegram token request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	req.Header.Set(
		"X-Service-Token",
		client.authServiceSecret,
	)

	resp, err :=
		client.httpClient.Do(req)

	if err != nil {
		return "", fmt.Errorf(
			"telegram token request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode !=
		http.StatusOK {

		apiErr, rawBody :=
			readAPIError(resp)

		switch apiErr.Code {
		case "telegram_not_linked":
			return "",
				ErrTelegramNotLinked

		case "invalid_telegram_user_id":
			return "",
				ErrInvalidTelegramUserID

		case "unauthorized":
			return "",
				ErrUnauthorizedService

		default:
			return "", fmt.Errorf(
				"auth-service returned status %d, code=%s, message=%s: %s",
				resp.StatusCode,
				apiErr.Code,
				apiErr.Message,
				rawBody,
			)
		}
	}

	var responseBody struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}

	err =
		json.NewDecoder(
			resp.Body,
		).Decode(
			&responseBody,
		)

	if err != nil {
		return "", fmt.Errorf(
			"decode telegram token response: %w",
			err,
		)
	}

	if strings.TrimSpace(
		responseBody.AccessToken,
	) == "" {
		return "", errors.New(
			"auth-service returned empty access token",
		)
	}

	return responseBody.AccessToken, nil
}

func (client *Client) getTelegramAccessToken(
	ctx context.Context,
	telegramUserID int64,
) (string, error) {
	return client.TelegramToken(
		ctx,
		telegramUserID,
	)
}

func (client *Client) ListMovies(
	ctx context.Context,
	telegramUserID int64,
) ([]Movie, error) {
	accessToken, err :=
		client.TelegramToken(
			ctx,
			telegramUserID,
		)

	if err != nil {
		return nil, err
	}

	req, err :=
		client.newLibraryRequest(
			ctx,
			http.MethodGet,
			"/movies",
			accessToken,
		)

	if err != nil {
		return nil, err
	}

	resp, err :=
		client.httpClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf(
			"library movie list request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode !=
		http.StatusOK {

		apiErr, rawBody :=
			readAPIError(resp)

		if resp.StatusCode ==
			http.StatusUnauthorized {

			return nil,
				ErrLibraryUnauthorized
		}

		return nil, fmt.Errorf(
			"library-service returned status %d, code=%s, message=%s: %s",
			resp.StatusCode,
			apiErr.Code,
			apiErr.Message,
			rawBody,
		)
	}

	var movies []Movie

	err =
		json.NewDecoder(
			io.LimitReader(
				resp.Body,
				4*1024*1024,
			),
		).Decode(
			&movies,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"decode movie list: %w",
			err,
		)
	}

	return movies, nil
}

func (client *Client) GetMovies(
	ctx context.Context,
	telegramUserID int64,
) ([]Movie, error) {
	return client.ListMovies(
		ctx,
		telegramUserID,
	)
}

func (client *Client) RandomMovie(
	ctx context.Context,
	telegramUserID int64,
) (*Movie, error) {
	accessToken, err :=
		client.TelegramToken(
			ctx,
			telegramUserID,
		)

	if err != nil {
		return nil, err
	}

	req, err :=
		client.newLibraryRequest(
			ctx,
			http.MethodGet,
			"/movies/random",
			accessToken,
		)

	if err != nil {
		return nil, err
	}

	resp, err :=
		client.httpClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf(
			"library random movie request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode !=
		http.StatusOK {

		apiErr, rawBody :=
			readAPIError(resp)

		switch {
		case resp.StatusCode ==
			http.StatusNotFound:

			return nil,
				ErrMovieNotFound

		case resp.StatusCode ==
			http.StatusUnauthorized:

			return nil,
				ErrLibraryUnauthorized

		default:
			return nil, fmt.Errorf(
				"library-service returned status %d, code=%s, message=%s: %s",
				resp.StatusCode,
				apiErr.Code,
				apiErr.Message,
				rawBody,
			)
		}
	}

	var movie Movie

	err =
		json.NewDecoder(
			io.LimitReader(
				resp.Body,
				1024*1024,
			),
		).Decode(
			&movie,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"decode random movie: %w",
			err,
		)
	}

	return &movie, nil
}

func (client *Client) GetRandomMovie(
	ctx context.Context,
	telegramUserID int64,
) (*Movie, error) {
	return client.RandomMovie(
		ctx,
		telegramUserID,
	)
}

func (client *Client) newLibraryRequest(
	ctx context.Context,
	method string,
	path string,
	accessToken string,
) (*http.Request, error) {
	requestURL :=
		client.libraryURL +
			path

	req, err :=
		http.NewRequestWithContext(
			ctx,
			method,
			requestURL,
			nil,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"create library request: %w",
			err,
		)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	return req, nil
}

func (client *Client) checkReady(
	ctx context.Context,
	baseURL string,
) error {
	requestURL :=
		baseURL + "/ready"

	req, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			requestURL,
			nil,
		)

	if err != nil {
		return fmt.Errorf(
			"create ready request: %w",
			err,
		)
	}

	resp, err :=
		client.httpClient.Do(req)

	if err != nil {
		return fmt.Errorf(
			"ready request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode !=
		http.StatusOK {

		body, _ :=
			io.ReadAll(
				io.LimitReader(
					resp.Body,
					1024,
				),
			)

		return fmt.Errorf(
			"service returned status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(
				string(body),
			),
		)
	}

	return nil
}

func readAPIError(
	resp *http.Response,
) (apiError, string) {
	body, err :=
		io.ReadAll(
			io.LimitReader(
				resp.Body,
				4096,
			),
		)

	if err != nil {
		return apiError{}, ""
	}

	rawBody :=
		strings.TrimSpace(
			string(body),
		)

	var response apiErrorResponse

	_ =
		json.Unmarshal(
			body,
			&response,
		)

	return response.Error,
		rawBody
}

func decodeAPIError(
	reader io.Reader,
) (apiError, error) {
	var response apiErrorResponse

	err :=
		json.NewDecoder(
			io.LimitReader(
				reader,
				4096,
			),
		).Decode(
			&response,
		)

	if err != nil {
		return apiError{}, err
	}

	return response.Error, nil
}
