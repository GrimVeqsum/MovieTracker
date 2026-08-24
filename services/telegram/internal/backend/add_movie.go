package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	ErrMovieAlreadyExists = errors.New("movie already exists")
	ErrMovieTitleRequired = errors.New("movie title is required")
)

func (client *Client) AddMovie(
	ctx context.Context,
	telegramUserID int64,
	title string,
	releaseYear *int,
) (*Movie, error) {
	accessToken, err := client.getTelegramAccessToken(
		ctx,
		telegramUserID,
	)
	if err != nil {
		return nil, err
	}

	requestBody := struct {
		Title       string `json:"title"`
		ReleaseYear *int   `json:"release_year,omitempty"`
	}{
		Title:       title,
		ReleaseYear: releaseYear,
	}

	data, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal create movie request: %w",
			err,
		)
	}

	requestURL := client.libraryURL + "/movies"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		requestURL,
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create movie request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"create movie request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		apiErr, decodeErr := decodeAPIError(resp.Body)

		if decodeErr != nil {
			return nil, fmt.Errorf(
				"library-service returned status %d",
				resp.StatusCode,
			)
		}

		switch apiErr.Code {
		case "movie_already_exists":
			return nil, ErrMovieAlreadyExists

		case "movie_title_required":
			return nil, ErrMovieTitleRequired

		case "unauthorized":
			return nil, ErrUnauthorizedService

		default:
			return nil, fmt.Errorf(
				"library-service returned %s: %s",
				apiErr.Code,
				apiErr.Message,
			)
		}
	}

	var movie Movie

	err = json.NewDecoder(
		io.LimitReader(
			resp.Body,
			1024*1024,
		),
	).Decode(&movie)

	if err != nil {
		return nil, fmt.Errorf(
			"decode created movie: %w",
			err,
		)
	}

	return &movie, nil
}
