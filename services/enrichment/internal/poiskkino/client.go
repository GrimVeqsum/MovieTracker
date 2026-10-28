package poiskkino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.poiskkino.dev/v1.4"

var ErrMovieNotFound = errors.New("movie not found")

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return NewClientWithBaseURL(apiKey, defaultBaseURL)
}

// NewClientWithBaseURL allows an isolated local API in integration tests.
func NewClientWithBaseURL(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			// An API redirect must not forward our API key to another host.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
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
		client.baseURL + "/movie/search",
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

	detailsURL := client.baseURL +
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

	if movie.ID <= 0 {
		return nil, errors.New("PoiskKino returned invalid movie details")
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

	const maxResponseBytes = 2 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return errors.New("PoiskKino response exceeds size limit")
	}
	if err := json.Unmarshal(data, result); err != nil {
		return err
	}

	return nil
}
