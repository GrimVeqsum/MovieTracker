package bot

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"movie-platform/telegram/internal/telegram"
)

type BackendClient interface {
	AuthReady(
		ctx context.Context,
	) error

	LibraryReady(
		ctx context.Context,
	) error
}

type Bot struct {
	telegramClient *telegram.Client
	backendClient  BackendClient
}

func New(
	telegramClient *telegram.Client,
	backendClient BackendClient,
) *Bot {
	return &Bot{
		telegramClient: telegramClient,
		backendClient:  backendClient,
	}
}

func (bot *Bot) Run(
	ctx context.Context,
) error {
	var offset int64

	for {
		if ctx.Err() != nil {
			return nil
		}

		updates, err :=
			bot.telegramClient.GetUpdates(
				ctx,
				offset,
			)

		if err != nil {
			if errors.Is(
				err,
				context.Canceled,
			) {
				return nil
			}

			log.Printf(
				"getUpdates error: %v",
				err,
			)

			select {
			case <-ctx.Done():
				return nil

			case <-time.After(
				2 * time.Second,
			):
				continue
			}
		}

		for _, update := range updates {
			offset = update.UpdateID + 1

			if update.Message == nil {
				continue
			}

			if err := bot.handleMessage(
				ctx,
				update.Message,
			); err != nil {
				log.Printf(
					"handle message error: %v",
					err,
				)
			}
		}
	}
}

func (bot *Bot) handleMessage(
	ctx context.Context,
	message *telegram.Message,
) error {
	text := strings.TrimSpace(
		message.Text,
	)

	if text == "" {
		return nil
	}

	log.Printf(
		"telegram message: chat_id=%d text=%q",
		message.Chat.ID,
		text,
	)

	switch text {
	case "/start":
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"MovieTracker Bot запущен.\n\n"+
				"Команды:\n"+
				"/ping — проверить Telegram bot\n"+
				"/auth — проверить auth-service\n"+
				"/library — проверить library-service\n"+
				"/status — проверить backend",
		)

	case "/help":
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Команды:\n"+
				"/ping\n"+
				"/auth\n"+
				"/library\n"+
				"/status",
		)

	case "/ping":
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"pong",
		)

	case "/auth":
		return bot.handleAuthStatus(
			ctx,
			message.Chat.ID,
		)

	case "/library":
		return bot.handleLibraryStatus(
			ctx,
			message.Chat.ID,
		)

	case "/status":
		return bot.handleBackendStatus(
			ctx,
			message.Chat.ID,
		)

	default:
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Неизвестная команда. Используй /help.",
		)
	}
}

func (bot *Bot) handleAuthStatus(
	ctx context.Context,
	chatID int64,
) error {
	if err := bot.backendClient.AuthReady(
		ctx,
	); err != nil {
		log.Printf(
			"auth-service unavailable: %v",
			err,
		)

		return bot.telegramClient.SendMessage(
			ctx,
			chatID,
			"auth-service: unavailable",
		)
	}

	return bot.telegramClient.SendMessage(
		ctx,
		chatID,
		"auth-service: ready",
	)
}

func (bot *Bot) handleLibraryStatus(
	ctx context.Context,
	chatID int64,
) error {
	if err := bot.backendClient.LibraryReady(
		ctx,
	); err != nil {
		log.Printf(
			"library-service unavailable: %v",
			err,
		)

		return bot.telegramClient.SendMessage(
			ctx,
			chatID,
			"library-service: unavailable",
		)
	}

	return bot.telegramClient.SendMessage(
		ctx,
		chatID,
		"library-service: ready",
	)
}

func (bot *Bot) handleBackendStatus(
	ctx context.Context,
	chatID int64,
) error {
	authErr :=
		bot.backendClient.AuthReady(ctx)

	libraryErr :=
		bot.backendClient.LibraryReady(ctx)

	authStatus := "ready"
	libraryStatus := "ready"

	if authErr != nil {
		authStatus = "unavailable"

		log.Printf(
			"auth-service unavailable: %v",
			authErr,
		)
	}

	if libraryErr != nil {
		libraryStatus = "unavailable"

		log.Printf(
			"library-service unavailable: %v",
			libraryErr,
		)
	}

	message :=
		"MovieTracker backend:\n" +
			"auth-service: " +
			authStatus +
			"\n" +
			"library-service: " +
			libraryStatus

	return bot.telegramClient.SendMessage(
		ctx,
		chatID,
		message,
	)
}
