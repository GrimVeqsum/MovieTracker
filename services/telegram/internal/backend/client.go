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
		"telegram account is not linked",
	)

	ErrMovieNotFound = errors.New(
		"movie not found",
	)
)

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Movie struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ReleaseYear *int   `json:"release_year"`

	OriginalTitle  *string `json:"original_title,omitempty"`
	Description    *string `json:"description,omitempty"`
	PosterURL      *string `json:"poster_url,omitempty"`
	RuntimeMinutes *int    `json:"runtime_minutes,omitempty"`

	MetadataStatus string `json:"metadata_status"`

	Genres []Genre `json:"genres,omitempty"`

	Status string  `json:"status"`
	Rating *int    `json:"rating"`
	Review *string `json:"review"`
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
		Code:           code,
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

	apiErr, err :=
		decodeAPIError(resp.Body)

	if err != nil {
		return fmt.Errorf(
			"auth-service returned status %d",
			resp.StatusCode,
		)
	}

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
			"auth-service returned %s: %s",
			apiErr.Code,
			apiErr.Message,
		)
	}
}

func (client *Client) GetMovies(
	ctx context.Context,
	telegramUserID int64,
) ([]Movie, error) {
	accessToken, err :=
		client.getTelegramAccessToken(
			ctx,
			telegramUserID,
		)

	if err != nil {
		return nil, err
	}

	requestURL :=
		client.libraryURL +
			"/movies"

	req, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			requestURL,
			nil,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"create movies request: %w",
				err,
			)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	resp, err :=
		client.httpClient.Do(req)

	if err != nil {
		return nil,
			fmt.Errorf(
				"movies request failed: %w",
				err,
			)
	}

	defer resp.Body.Close()

	if resp.StatusCode !=
		http.StatusOK {

		apiErr, decodeErr :=
			decodeAPIError(
				resp.Body,
			)

		if decodeErr == nil {
			return nil,
				fmt.Errorf(
					"library-service returned %s: %s",
					apiErr.Code,
					apiErr.Message,
				)
		}

		return nil,
			fmt.Errorf(
				"library-service returned status %d",
				resp.StatusCode,
			)
	}

	var movies []Movie

	err =
		json.NewDecoder(
			io.LimitReader(
				resp.Body,
				1024*1024,
			),
		).Decode(
			&movies,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"decode movies response: %w",
				err,
			)
	}

	return movies, nil
}

func (client *Client) GetRandomMovie(
	ctx context.Context,
	telegramUserID int64,
) (*Movie, error) {
	accessToken, err :=
		client.getTelegramAccessToken(
			ctx,
			telegramUserID,
		)

	if err != nil {
		return nil, err
	}

	requestURL :=
		client.libraryURL +
			"/movies/random"

	req, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			requestURL,
			nil,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"create random movie request: %w",
				err,
			)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	resp, err :=
		client.httpClient.Do(req)

	if err != nil {
		return nil,
			fmt.Errorf(
				"random movie request failed: %w",
				err,
			)
	}

	defer resp.Body.Close()

	if resp.StatusCode ==
		http.StatusNotFound {

		return nil,
			ErrMovieNotFound
	}

	if resp.StatusCode !=
		http.StatusOK {

		apiErr, decodeErr :=
			decodeAPIError(
				resp.Body,
			)

		if decodeErr == nil {
			return nil,
				fmt.Errorf(
					"library-service returned %s: %s",
					apiErr.Code,
					apiErr.Message,
				)
		}

		return nil,
			fmt.Errorf(
				"library-service returned status %d",
				resp.StatusCode,
			)
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
		return nil,
			fmt.Errorf(
				"decode random movie response: %w",
				err,
			)
	}

	return &movie, nil
}

func (client *Client) getTelegramAccessToken(
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
		return "",
			fmt.Errorf(
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
		return "",
			fmt.Errorf(
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
		return "",
			fmt.Errorf(
				"telegram token request failed: %w",
				err,
			)
	}

	defer resp.Body.Close()

	if resp.StatusCode !=
		http.StatusOK {

		apiErr, decodeErr :=
			decodeAPIError(
				resp.Body,
			)

		if decodeErr != nil {
			return "",
				fmt.Errorf(
					"auth-service returned status %d",
					resp.StatusCode,
				)
		}

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
			return "",
				fmt.Errorf(
					"auth-service returned %s: %s",
					apiErr.Code,
					apiErr.Message,
				)
		}
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}

	err =
		json.NewDecoder(
			io.LimitReader(
				resp.Body,
				4096,
			),
		).Decode(
			&tokenResponse,
		)

	if err != nil {
		return "",
			fmt.Errorf(
				"decode telegram token response: %w",
				err,
			)
	}

	if strings.TrimSpace(
		tokenResponse.AccessToken,
	) == "" {
		return "",
			errors.New(
				"auth-service returned empty access token",
			)
	}

	return tokenResponse.AccessToken,
		nil
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

type apiError struct {
	Code    string
	Message string
}

func decodeAPIError(
	body io.Reader,
) (apiError, error) {
	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	err :=
		json.NewDecoder(
			io.LimitReader(
				body,
				4096,
			),
		).Decode(
			&response,
		)

	if err != nil {
		return apiError{}, err
	}

	return apiError{
		Code:    response.Error.Code,
		Message: response.Error.Message,
	}, nil
}
