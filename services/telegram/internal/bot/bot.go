package bot

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"movie-platform/telegram/internal/backend"
	"movie-platform/telegram/internal/telegram"
)

type BackendClient interface {
	AuthReady(
		ctx context.Context,
	) error

	LibraryReady(
		ctx context.Context,
	) error

	LinkTelegram(
		ctx context.Context,
		code string,
		telegramUserID int64,
	) error

	AddMovie(
		ctx context.Context,
		telegramUserID int64,
		title string,
		releaseYear *int,
	) (*backend.Movie, error)

	GetMovies(
		ctx context.Context,
		telegramUserID int64,
	) ([]backend.Movie, error)

	GetRandomMovie(
		ctx context.Context,
		telegramUserID int64,
	) (*backend.Movie, error)

	MakeWatched(
		ctx context.Context,
		telegramUserID int64,
		movieID string,
		rating int,
		review *string,
	) (*backend.Movie, error)

	MakeUnwatched(
		ctx context.Context,
		telegramUserID int64,
		movieID string,
	) (*backend.Movie, error)

	DeleteMovie(
		ctx context.Context,
		telegramUserID int64,
		movieID string,
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

			offset =
				update.UpdateID + 1

			if update.Message == nil {
				continue
			}

			if err :=
				bot.handleMessage(
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
	text :=
		strings.TrimSpace(
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

	fields :=
		strings.Fields(text)

	if len(fields) == 0 {
		return nil
	}

	command :=
		strings.ToLower(
			fields[0],
		)

	if index :=
		strings.Index(
			command,
			"@",
		); index >= 0 {

		command =
			command[:index]
	}

	switch command {
	case "/start":
		return bot.handleStart(
			ctx,
			message,
			fields,
		)

	case "/help":
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"MovieTracker Bot\n\n"+
				"/add НАЗВАНИЕ — добавить фильм\n"+
				"/add НАЗВАНИЕ | ГОД — добавить фильм с годом\n"+
				"/movies — список фильмов\n"+
				"/random — случайный фильм\n"+
				"/watched N RATING — отметить просмотренным\n"+
				"/watched N RATING | REVIEW — добавить отзыв\n"+
				"/unwatched N — вернуть в непросмотренные\n"+
				"/delete N — удалить фильм\n"+
				"/link CODE — привязать аккаунт\n"+
				"/status — проверить backend",
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

	case "/link":
		return bot.handleLink(
			ctx,
			message,
			fields,
		)

	case "/add":
		return bot.handleAdd(
			ctx,
			message,
			text,
		)

	case "/movies":
		return bot.handleMovies(
			ctx,
			message,
		)

	case "/random":
		return bot.handleRandom(
			ctx,
			message,
		)

	case "/watched":
		return bot.handleWatched(
			ctx,
			message,
			text,
		)

	case "/unwatched":
		return bot.handleUnwatched(
			ctx,
			message,
			text,
		)

	case "/delete":
		return bot.handleDelete(
			ctx,
			message,
			text,
		)

	default:
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Неизвестная команда. Используй /help.",
		)
	}
}
