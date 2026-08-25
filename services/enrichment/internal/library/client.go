package library

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const serviceTokenHeader = "X-Service-Token"

type Client struct {
	baseURL string

	serviceSecret string

	httpClient *http.Client
}

func NewClient(
	baseURL string,
	serviceSecret string,
) *Client {
	return &Client{
		baseURL: strings.TrimRight(
			baseURL,
			"/",
		),

		serviceSecret: strings.TrimSpace(
			serviceSecret,
		),

		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type UpdateMetadataRequest struct {
	EventID string `json:"event_id"`

	UserID string `json:"user_id"`

	ExternalID string `json:"external_id"`

	MetadataProvider string `json:"metadata_provider"`

	OriginalTitle string `json:"original_title"`

	Description string `json:"description"`

	ReleaseYear int `json:"release_year"`

	PosterURL string `json:"poster_url"`

	RuntimeMinutes *int `json:"runtime_minutes"`

	Genres []string `json:"genres"`
}

func (client *Client) UpdateMetadata(
	ctx context.Context,
	movieID string,
	request UpdateMetadataRequest,
) error {
	requestURL :=
		fmt.Sprintf(
			"%s/internal/movies/%s/metadata",
			client.baseURL,
			movieID,
		)

	return client.patchJSON(
		ctx,
		requestURL,
		request,
	)
}

type MarkMetadataFailedRequest struct {
	EventID string `json:"event_id"`

	UserID string `json:"user_id"`

	Error string `json:"error"`
}

func (client *Client) MarkMetadataFailed(
	ctx context.Context,
	movieID string,
	request MarkMetadataFailedRequest,
) error {
	requestURL :=
		fmt.Sprintf(
			"%s/internal/movies/%s/metadata/failed",
			client.baseURL,
			movieID,
		)

	return client.patchJSON(
		ctx,
		requestURL,
		request,
	)
}

func (client *Client) patchJSON(
	ctx context.Context,
	requestURL string,
	body any,
) error {
	data, err :=
		json.Marshal(
			body,
		)

	if err != nil {
		return fmt.Errorf(
			"marshal library request: %w",
			err,
		)
	}

	req, err :=
		http.NewRequestWithContext(
			ctx,
			http.MethodPatch,
			requestURL,
			bytes.NewReader(
				data,
			),
		)

	if err != nil {
		return fmt.Errorf(
			"create library request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	req.Header.Set(
		serviceTokenHeader,
		client.serviceSecret,
	)

	resp, err :=
		client.httpClient.Do(
			req,
		)

	if err != nil {
		return fmt.Errorf(
			"library request: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode ==
		http.StatusNoContent {

		return nil
	}

	errorBody, _ :=
		io.ReadAll(
			io.LimitReader(
				resp.Body,
				4096,
			),
		)

	message :=
		strings.TrimSpace(
			string(errorBody),
		)

	if message == "" {
		return fmt.Errorf(
			"library returned status %d",
			resp.StatusCode,
		)
	}

	return fmt.Errorf(
		"library returned status %d: %s",
		resp.StatusCode,
		message,
	)
}
