package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	authURL    string
	libraryURL string
	httpClient *http.Client
}

func NewClient(
	authURL string,
	libraryURL string,
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

func (client *Client) checkReady(
	ctx context.Context,
	baseURL string,
) error {
	requestURL := baseURL + "/ready"

	req, err := http.NewRequestWithContext(
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

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"ready request failed: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(
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
