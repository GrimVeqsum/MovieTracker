package poiskkino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const baseURL = "https://api.poiskkino.dev/v1.4"

var ErrMovieNotFound = errors.New("movie not found")

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type Genre struct {
	Name string `json:"name"`
}

type Poster struct {
	URL        string `json:"url"`
	PreviewURL string `json:"previewUrl"`
}

type SearchMovie struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	AlternativeName string `json:"alternativeName"`
	Year            int    `json:"year"`
}

type SearchResponse struct {
	Docs []SearchMovie `json:"docs"`
}

type MovieDetails struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	AlternativeName string  `json:"alternativeName"`
	Description     string  `json:"description"`
	Year            int     `json:"year"`
	MovieLength     *int    `json:"movieLength"`
	Poster          Poster  `json:"poster"`
	Genres          []Genre `json:"genres"`
}

func (client *Client) FindMovie(
	ctx context.Context,
	title string,
	releaseYear *int,
) (*MovieDetails, error) {
	searchURL, err := url.Parse(
		baseURL + "/movie/search",
	)
	if err != nil {
		return nil, err
	}

	query := searchURL.Query()

	query.Set("query", title)
	query.Set("page", "1")
	query.Set("limit", "10")

	searchURL.RawQuery = query.Encode()

	var searchResponse SearchResponse

	err = client.get(
		ctx,
		searchURL.String(),
		&searchResponse,
	)
	if err != nil {
		return nil, err
	}

	if len(searchResponse.Docs) == 0 {
		return nil, ErrMovieNotFound
	}

	movieID := searchResponse.Docs[0].ID

	// Если пользователь указал год, стараемся найти
	// фильм именно этого года среди результатов.
	if releaseYear != nil {
		found := false

		for _, movie := range searchResponse.Docs {
			if movie.Year == *releaseYear {
				movieID = movie.ID
				found = true
				break
			}
		}

		if !found {
			return nil, ErrMovieNotFound
		}
	}

	detailsURL := baseURL +
		"/movie/" +
		strconv.Itoa(movieID)

	var movie MovieDetails

	err = client.get(
		ctx,
		detailsURL,
		&movie,
	)
	if err != nil {
		return nil, err
	}

	return &movie, nil
}

func (client *Client) get(
	ctx context.Context,
	requestURL string,
	result any,
) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return err
	}

	req.Header.Set(
		"X-API-KEY",
		client.apiKey,
	)

	req.Header.Set(
		"Accept",
		"application/json",
	)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return errors.New("invalid PoiskKino API key")
	case http.StatusForbidden:
		return errors.New("PoiskKino API access forbidden")
	case http.StatusTooManyRequests:
		return errors.New("PoiskKino API rate limit exceeded")
	default:
		return fmt.Errorf(
			"PoiskKino API returned status %d",
			resp.StatusCode,
		)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return err
	}

	return nil
}
