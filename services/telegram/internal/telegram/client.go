package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

type getMeResponse struct {
	OK          bool   `json:"ok"`
	Result      User   `json:"result"`
	Description string `json:"description"`
}

type getUpdatesResponse struct {
	OK          bool     `json:"ok"`
	Result      []Update `json:"result"`
	Description string   `json:"description"`
}

type sendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func NewClient(token string) *Client {
	return &Client{
		baseURL: fmt.Sprintf(
			"https://api.telegram.org/bot%s",
			token,
		),

		httpClient: &http.Client{
			Timeout: 40 * time.Second,
		},
	}
}

func (client *Client) GetMe(
	ctx context.Context,
) (*User, error) {
	requestURL := client.baseURL + "/getMe"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create getMe request: %w",
			err,
		)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"getMe request failed: %w",
			err,
		)
	}
	defer resp.Body.Close()

	var response getMeResponse

	if err := json.NewDecoder(
		resp.Body,
	).Decode(&response); err != nil {
		return nil, fmt.Errorf(
			"decode getMe response: %w",
			err,
		)
	}

	if !response.OK {
		return nil, fmt.Errorf(
			"Telegram getMe failed: %s",
			response.Description,
		)
	}

	return &response.Result, nil
}

func (client *Client) GetUpdates(
	ctx context.Context,
	offset int64,
) ([]Update, error) {
	params := url.Values{}

	params.Set(
		"timeout",
		"30",
	)

	if offset > 0 {
		params.Set(
			"offset",
			strconv.FormatInt(
				offset,
				10,
			),
		)
	}

	requestURL := fmt.Sprintf(
		"%s/getUpdates?%s",
		client.baseURL,
		params.Encode(),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		requestURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create getUpdates request: %w",
			err,
		)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		if errors.Is(
			err,
			context.Canceled,
		) {
			return nil, context.Canceled
		}

		return nil, fmt.Errorf(
			"getUpdates request failed: %w",
			err,
		)
	}
	defer resp.Body.Close()

	var response getUpdatesResponse

	if err := json.NewDecoder(
		resp.Body,
	).Decode(&response); err != nil {
		return nil, fmt.Errorf(
			"decode getUpdates response: %w",
			err,
		)
	}

	if !response.OK {
		return nil, fmt.Errorf(
			"Telegram getUpdates failed: %s",
			response.Description,
		)
	}

	return response.Result, nil
}

func (client *Client) SendMessage(
	ctx context.Context,
	chatID int64,
	text string,
) error {
	data, err := json.Marshal(
		sendMessageRequest{
			ChatID: chatID,
			Text:   text,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"marshal sendMessage request: %w",
			err,
		)
	}

	requestURL :=
		client.baseURL + "/sendMessage"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		requestURL,
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf(
			"create sendMessage request: %w",
			err,
		)
	}

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"sendMessage request failed: %w",
			err,
		)
	}
	defer resp.Body.Close()

	var response apiResponse

	if err := json.NewDecoder(
		resp.Body,
	).Decode(&response); err != nil {
		return fmt.Errorf(
			"decode sendMessage response: %w",
			err,
		)
	}

	if !response.OK {
		return fmt.Errorf(
			"Telegram sendMessage failed: %s",
			response.Description,
		)
	}

	return nil
}
