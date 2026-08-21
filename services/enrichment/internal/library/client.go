package library

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type UpdateMetadataRequest struct {
	UserID           string   `json:"user_id"`
	ExternalID       string   `json:"external_id"`
	MetadataProvider string   `json:"metadata_provider"`
	OriginalTitle    string   `json:"original_title"`
	Description      string   `json:"description"`
	ReleaseYear      int      `json:"release_year"`
	PosterURL        string   `json:"poster_url"`
	RuntimeMinutes   *int     `json:"runtime_minutes"`
	Genres           []string `json:"genres"`
}

func (client *Client) UpdateMetadata(
	ctx context.Context,
	movieID string,
	request UpdateMetadataRequest,
) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}

	requestURL := fmt.Sprintf(
		"%s/internal/movies/%s/metadata",
		client.baseURL,
		movieID,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPatch,
		requestURL,
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf(
			"library returned status %d",
			resp.StatusCode,
		)
	}

	return nil
}
