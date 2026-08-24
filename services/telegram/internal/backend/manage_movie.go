package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

var ErrRatingOutOfRange = errors.New(
	"rating is out of range",
)

func (client *Client) MakeWatched(
	ctx context.Context,
	telegramUserID int64,
	movieID string,
	rating int,
	review *string,
) (*Movie, error) {
	accessToken, err :=
		client.getTelegramAccessToken(
			ctx,
			telegramUserID,
		)
	if err != nil {
		return nil, err
	}

	requestBody := struct {
		Rating int     `json:"rating"`
		Review *string `json:"review,omitempty"`
	}{
		Rating: rating,
		Review: review,
	}

	data, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal watched request: %w",
			err,
		)
	}

	requestURL :=
		client.libraryURL +
			"/movies/" +
			url.PathEscape(movieID) +
			"/watched"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPatch,
		requestURL,
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create watched request: %w",
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
			"watched request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		apiErr, decodeErr :=
			decodeAPIError(resp.Body)

		if decodeErr != nil {
			return nil, fmt.Errorf(
				"library-service returned status %d",
				resp.StatusCode,
			)
		}

		switch apiErr.Code {
		case "movie_not_found":
			return nil, ErrMovieNotFound

		case "rating_out_of_range":
			return nil, ErrRatingOutOfRange

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
			"decode watched movie: %w",
			err,
		)
	}

	return &movie, nil
}

func (client *Client) MakeUnwatched(
	ctx context.Context,
	telegramUserID int64,
	movieID string,
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
			"/movies/" +
			url.PathEscape(movieID) +
			"/unwatched"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPatch,
		requestURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create unwatched request: %w",
			err,
		)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"unwatched request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		apiErr, decodeErr :=
			decodeAPIError(resp.Body)

		if decodeErr != nil {
			return nil, fmt.Errorf(
				"library-service returned status %d",
				resp.StatusCode,
			)
		}

		switch apiErr.Code {
		case "movie_not_found":
			return nil, ErrMovieNotFound

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
			"decode unwatched movie: %w",
			err,
		)
	}

	return &movie, nil
}

func (client *Client) DeleteMovie(
	ctx context.Context,
	telegramUserID int64,
	movieID string,
) error {
	accessToken, err :=
		client.getTelegramAccessToken(
			ctx,
			telegramUserID,
		)
	if err != nil {
		return err
	}

	requestURL :=
		client.libraryURL +
			"/movies/" +
			url.PathEscape(movieID)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		requestURL,
		nil,
	)
	if err != nil {
		return fmt.Errorf(
			"create delete movie request: %w",
			err,
		)
	}

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"delete movie request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusNoContent {
		return nil
	}

	apiErr, decodeErr :=
		decodeAPIError(resp.Body)

	if decodeErr != nil {
		return fmt.Errorf(
			"library-service returned status %d",
			resp.StatusCode,
		)
	}

	switch apiErr.Code {
	case "movie_not_found":
		return ErrMovieNotFound

	case "unauthorized":
		return ErrUnauthorizedService

	default:
		return fmt.Errorf(
			"library-service returned %s: %s",
			apiErr.Code,
			apiErr.Message,
		)
	}
}
