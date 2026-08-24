package bot

import (
	"context"
	"errors"
	"log"
	"strings"

	"movie-platform/telegram/internal/backend"
	"movie-platform/telegram/internal/telegram"
)

func (bot *Bot) handleStart(
	ctx context.Context,
	message *telegram.Message,
	fields []string,
) error {
	if len(fields) == 2 {
		parameter :=
			strings.TrimSpace(
				fields[1],
			)

		if strings.HasPrefix(
			parameter,
			"link_",
		) {
			code :=
				strings.TrimPrefix(
					parameter,
					"link_",
				)

			return bot.linkTelegram(
				ctx,
				message,
				code,
			)
		}
	}

	return bot.telegramClient.SendMessage(
		ctx,
		message.Chat.ID,
		"MovieTracker Bot\n\n"+
			"/add НАЗВАНИЕ — добавить фильм\n"+
			"/movies — список фильмов\n"+
			"/random — случайный фильм\n"+
			"/watched N RATING — отметить просмотренным\n"+
			"/unwatched N — вернуть в непросмотренные\n"+
			"/delete N — удалить фильм\n"+
			"/link CODE — привязать аккаунт\n"+
			"/help — помощь",
	)
}

func (bot *Bot) handleLink(
	ctx context.Context,
	message *telegram.Message,
	fields []string,
) error {
	if len(fields) != 2 {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Использование: /link CODE",
		)
	}

	code :=
		strings.TrimSpace(
			fields[1],
		)

	return bot.linkTelegram(
		ctx,
		message,
		code,
	)
}

func (bot *Bot) linkTelegram(
	ctx context.Context,
	message *telegram.Message,
	code string,
) error {
	if message.From == nil {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Не удалось определить Telegram-пользователя.",
		)
	}

	if strings.TrimSpace(code) == "" {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Код привязки пуст.",
		)
	}

	err :=
		bot.backendClient.LinkTelegram(
			ctx,
			code,
			message.From.ID,
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			backend.ErrInvalidLinkCode,
		):
			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Код недействителен или истёк. Получи новую ссылку в MovieTracker.",
			)

		case errors.Is(
			err,
			backend.ErrTelegramAlreadyLinked,
		):
			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Этот Telegram уже привязан к другому аккаунту.",
			)

		case errors.Is(
			err,
			backend.ErrAccountAlreadyLinked,
		):
			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Аккаунт MovieTracker уже привязан.",
			)

		default:
			log.Printf(
				"telegram link failed: %v",
				err,
			)

			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Не удалось привязать аккаунт.",
			)
		}
	}

	return bot.telegramClient.SendMessage(
		ctx,
		message.Chat.ID,
		"Аккаунт MovieTracker успешно привязан.\n\n"+
			"Теперь можешь использовать /movies, /add и /random.",
	)
}

func (bot *Bot) handleAuthStatus(
	ctx context.Context,
	chatID int64,
) error {
	if err :=
		bot.backendClient.AuthReady(
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
	if err :=
		bot.backendClient.LibraryReady(
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
		bot.backendClient.AuthReady(
			ctx,
		)

	libraryErr :=
		bot.backendClient.LibraryReady(
			ctx,
		)

	authStatus := "ready"
	libraryStatus := "ready"

	if authErr != nil {
		authStatus =
			"unavailable"
	}

	if libraryErr != nil {
		libraryStatus =
			"unavailable"
	}

	return bot.telegramClient.SendMessage(
		ctx,
		chatID,
		"MovieTracker backend:\n"+
			"auth-service: "+
			authStatus+
			"\n"+
			"library-service: "+
			libraryStatus,
	)
}
