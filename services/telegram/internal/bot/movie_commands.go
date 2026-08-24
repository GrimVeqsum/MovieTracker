package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"movie-platform/telegram/internal/backend"
	"movie-platform/telegram/internal/telegram"
)

var errMovieNumberOutOfRange = errors.New("movie number is out of range")

func (bot *Bot) handleAdd(
	ctx context.Context,
	message *telegram.Message,
	text string,
) error {
	if message.From == nil {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Не удалось определить Telegram-пользователя.",
		)
	}

	title, releaseYear, err :=
		parseAddArguments(text)

	if err != nil {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Использование:\n"+
				"/add Интерстеллар\n"+
				"/add Интерстеллар | 2014",
		)
	}

	movie, err :=
		bot.backendClient.AddMovie(
			ctx,
			message.From.ID,
			title,
			releaseYear,
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			backend.ErrTelegramNotLinked,
		):
			return bot.notLinked(
				ctx,
				message.Chat.ID,
			)

		case errors.Is(
			err,
			backend.ErrMovieAlreadyExists,
		):
			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Этот фильм уже есть в твоём списке.",
			)

		default:
			log.Printf(
				"add movie failed: %v",
				err,
			)

			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Не удалось добавить фильм.",
			)
		}
	}

	var result strings.Builder

	result.WriteString("Фильм добавлен: ")
	result.WriteString(movie.Title)

	if movie.ReleaseYear != nil {
		fmt.Fprintf(
			&result,
			" (%d)",
			*movie.ReleaseYear,
		)
	}

	result.WriteString(
		"\nДанные о фильме будут загружены автоматически.",
	)

	return bot.telegramClient.SendMessage(
		ctx,
		message.Chat.ID,
		result.String(),
	)
}

func (bot *Bot) handleMovies(
	ctx context.Context,
	message *telegram.Message,
) error {
	if message.From == nil {
		return nil
	}

	movies, err :=
		bot.backendClient.GetMovies(
			ctx,
			message.From.ID,
		)

	if err != nil {
		if errors.Is(
			err,
			backend.ErrTelegramNotLinked,
		) {
			return bot.notLinked(
				ctx,
				message.Chat.ID,
			)
		}

		log.Printf(
			"get movies failed: %v",
			err,
		)

		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Не удалось получить список фильмов.",
		)
	}

	if len(movies) == 0 {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Твой список фильмов пока пуст.",
		)
	}

	return bot.telegramClient.SendMessage(
		ctx,
		message.Chat.ID,
		formatMovieList(movies),
	)
}

func (bot *Bot) handleRandom(
	ctx context.Context,
	message *telegram.Message,
) error {
	if message.From == nil {
		return nil
	}

	movie, err :=
		bot.backendClient.GetRandomMovie(
			ctx,
			message.From.ID,
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			backend.ErrTelegramNotLinked,
		):
			return bot.notLinked(
				ctx,
				message.Chat.ID,
			)

		case errors.Is(
			err,
			backend.ErrMovieNotFound,
		):
			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Нет подходящих фильмов для случайного выбора.",
			)

		default:
			log.Printf(
				"get random movie failed: %v",
				err,
			)

			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Не удалось выбрать случайный фильм.",
			)
		}
	}

	return bot.telegramClient.SendMessage(
		ctx,
		message.Chat.ID,
		formatMovieDetails(movie),
	)
}

func (bot *Bot) handleWatched(
	ctx context.Context,
	message *telegram.Message,
	text string,
) error {
	if message.From == nil {
		return nil
	}

	number, rating, review, err :=
		parseWatchedArguments(text)

	if err != nil {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Использование:\n"+
				"/watched 1 9\n"+
				"/watched 1 9 | Отличный фильм\n\n"+
				"Рейтинг должен быть от 1 до 10.",
		)
	}

	movie, err :=
		bot.movieByNumber(
			ctx,
			message.From.ID,
			number,
		)

	if err != nil {
		return bot.handleMovieNumberError(
			ctx,
			message.Chat.ID,
			err,
		)
	}

	updated, err :=
		bot.backendClient.MakeWatched(
			ctx,
			message.From.ID,
			movie.ID,
			rating,
			review,
		)

	if err != nil {
		switch {
		case errors.Is(
			err,
			backend.ErrMovieNotFound,
		):
			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Фильм не найден.",
			)

		case errors.Is(
			err,
			backend.ErrRatingOutOfRange,
		):
			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Рейтинг должен быть от 1 до 10.",
			)

		default:
			log.Printf(
				"make watched failed: %v",
				err,
			)

			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Не удалось изменить фильм.",
			)
		}
	}

	return bot.telegramClient.SendMessage(
		ctx,
		message.Chat.ID,
		fmt.Sprintf(
			"%s отмечен как просмотренный.\nОценка: %d/10",
			updated.Title,
			rating,
		),
	)
}

func (bot *Bot) handleUnwatched(
	ctx context.Context,
	message *telegram.Message,
	text string,
) error {
	if message.From == nil {
		return nil
	}

	number, err :=
		parseMovieNumberArgument(text)

	if err != nil {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Использование: /unwatched 1",
		)
	}

	movie, err :=
		bot.movieByNumber(
			ctx,
			message.From.ID,
			number,
		)

	if err != nil {
		return bot.handleMovieNumberError(
			ctx,
			message.Chat.ID,
			err,
		)
	}

	updated, err :=
		bot.backendClient.MakeUnwatched(
			ctx,
			message.From.ID,
			movie.ID,
		)

	if err != nil {
		log.Printf(
			"make unwatched failed: %v",
			err,
		)

		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Не удалось изменить фильм.",
		)
	}

	return bot.telegramClient.SendMessage(
		ctx,
		message.Chat.ID,
		updated.Title+
			" снова отмечен как непросмотренный.",
	)
}

func (bot *Bot) handleDelete(
	ctx context.Context,
	message *telegram.Message,
	text string,
) error {
	if message.From == nil {
		return nil
	}

	number, err :=
		parseMovieNumberArgument(text)

	if err != nil {
		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Использование: /delete 1",
		)
	}

	movie, err :=
		bot.movieByNumber(
			ctx,
			message.From.ID,
			number,
		)

	if err != nil {
		return bot.handleMovieNumberError(
			ctx,
			message.Chat.ID,
			err,
		)
	}

	err =
		bot.backendClient.DeleteMovie(
			ctx,
			message.From.ID,
			movie.ID,
		)

	if err != nil {
		if errors.Is(
			err,
			backend.ErrMovieNotFound,
		) {
			return bot.telegramClient.SendMessage(
				ctx,
				message.Chat.ID,
				"Фильм не найден.",
			)
		}

		log.Printf(
			"delete movie failed: %v",
			err,
		)

		return bot.telegramClient.SendMessage(
			ctx,
			message.Chat.ID,
			"Не удалось удалить фильм.",
		)
	}

	return bot.telegramClient.SendMessage(
		ctx,
		message.Chat.ID,
		"Фильм удалён: "+movie.Title,
	)
}

func (bot *Bot) movieByNumber(
	ctx context.Context,
	telegramUserID int64,
	number int,
) (*backend.Movie, error) {
	movies, err :=
		bot.backendClient.GetMovies(
			ctx,
			telegramUserID,
		)

	if err != nil {
		return nil, err
	}

	if number < 1 ||
		number > len(movies) {
		return nil,
			errMovieNumberOutOfRange
	}

	return &movies[number-1], nil
}

func (bot *Bot) handleMovieNumberError(
	ctx context.Context,
	chatID int64,
	err error,
) error {
	switch {
	case errors.Is(
		err,
		backend.ErrTelegramNotLinked,
	):
		return bot.notLinked(
			ctx,
			chatID,
		)

	case errors.Is(
		err,
		errMovieNumberOutOfRange,
	):
		return bot.telegramClient.SendMessage(
			ctx,
			chatID,
			"Такого номера фильма нет. Посмотри актуальный список через /movies.",
		)

	default:
		log.Printf(
			"resolve movie number failed: %v",
			err,
		)

		return bot.telegramClient.SendMessage(
			ctx,
			chatID,
			"Не удалось получить фильм.",
		)
	}
}

func (bot *Bot) notLinked(
	ctx context.Context,
	chatID int64,
) error {
	return bot.telegramClient.SendMessage(
		ctx,
		chatID,
		"Telegram не привязан к MovieTracker. Сначала используй /link CODE.",
	)
}

func parseAddArguments(
	text string,
) (string, *int, error) {
	arguments := commandArguments(text)

	if arguments == "" {
		return "", nil,
			errors.New("movie title is missing")
	}

	title := arguments

	var releaseYear *int

	if strings.Contains(
		arguments,
		"|",
	) {
		parts :=
			strings.SplitN(
				arguments,
				"|",
				2,
			)

		title =
			strings.TrimSpace(
				parts[0],
			)

		yearText :=
			strings.TrimSpace(
				parts[1],
			)

		year, err :=
			strconv.Atoi(yearText)

		if err != nil {
			return "", nil, err
		}

		releaseYear = &year
	}

	if title == "" {
		return "", nil,
			errors.New("movie title is missing")
	}

	return title,
		releaseYear,
		nil
}

func parseWatchedArguments(
	text string,
) (int, int, *string, error) {
	arguments := commandArguments(text)

	if arguments == "" {
		return 0, 0, nil,
			errors.New("arguments are missing")
	}

	parts :=
		strings.SplitN(
			arguments,
			"|",
			2,
		)

	fields :=
		strings.Fields(
			parts[0],
		)

	if len(fields) != 2 {
		return 0, 0, nil,
			errors.New("invalid arguments")
	}

	number, err :=
		strconv.Atoi(fields[0])
	if err != nil || number < 1 {
		return 0, 0, nil,
			errors.New("invalid movie number")
	}

	rating, err :=
		strconv.Atoi(fields[1])
	if err != nil ||
		rating < 1 ||
		rating > 10 {
		return 0, 0, nil,
			errors.New("invalid rating")
	}

	var review *string

	if len(parts) == 2 {
		value :=
			strings.TrimSpace(
				parts[1],
			)

		if value != "" {
			review = &value
		}
	}

	return number,
		rating,
		review,
		nil
}

func parseMovieNumberArgument(
	text string,
) (int, error) {
	arguments := commandArguments(text)

	fields :=
		strings.Fields(arguments)

	if len(fields) != 1 {
		return 0,
			errors.New("invalid movie number")
	}

	number, err :=
		strconv.Atoi(fields[0])

	if err != nil ||
		number < 1 {
		return 0,
			errors.New("invalid movie number")
	}

	return number, nil
}

func commandArguments(
	text string,
) string {
	text = strings.TrimSpace(text)

	index :=
		strings.IndexAny(
			text,
			" \t\n",
		)

	if index == -1 {
		return ""
	}

	return strings.TrimSpace(
		text[index:],
	)
}

func formatMovieList(
	movies []backend.Movie,
) string {
	const maxMovies = 15

	limit := len(movies)

	if limit > maxMovies {
		limit = maxMovies
	}

	var builder strings.Builder

	fmt.Fprintf(
		&builder,
		"Твои фильмы: %d\n\n",
		len(movies),
	)

	for i := 0; i < limit; i++ {
		movie := movies[i]

		fmt.Fprintf(
			&builder,
			"%d. %s",
			i+1,
			truncate(
				movie.Title,
				80,
			),
		)

		if movie.ReleaseYear != nil {
			fmt.Fprintf(
				&builder,
				" (%d)",
				*movie.ReleaseYear,
			)
		}

		builder.WriteString(" — ")

		builder.WriteString(
			displayStatus(
				movie.Status,
			),
		)

		if movie.Rating != nil {
			fmt.Fprintf(
				&builder,
				", %d/10",
				*movie.Rating,
			)
		}

		builder.WriteString("\n")
	}

	if len(movies) > maxMovies {
		fmt.Fprintf(
			&builder,
			"\nПоказаны первые %d из %d.\n",
			maxMovies,
			len(movies),
		)
	}

	builder.WriteString(
		"\nПримеры:\n" +
			"/watched 1 9\n" +
			"/unwatched 1\n" +
			"/delete 1",
	)

	return builder.String()
}

func formatMovieDetails(
	movie *backend.Movie,
) string {
	var builder strings.Builder

	builder.WriteString(movie.Title)

	if movie.ReleaseYear != nil {
		fmt.Fprintf(
			&builder,
			" (%d)",
			*movie.ReleaseYear,
		)
	}

	builder.WriteString("\n")

	if movie.OriginalTitle != nil &&
		strings.TrimSpace(
			*movie.OriginalTitle,
		) != "" &&
		*movie.OriginalTitle != movie.Title {

		fmt.Fprintf(
			&builder,
			"Оригинальное название: %s\n",
			*movie.OriginalTitle,
		)
	}

	fmt.Fprintf(
		&builder,
		"Статус: %s\n",
		displayStatus(
			movie.Status,
		),
	)

	if movie.Rating != nil {
		fmt.Fprintf(
			&builder,
			"Оценка: %d/10\n",
			*movie.Rating,
		)
	}

	if movie.RuntimeMinutes != nil {
		fmt.Fprintf(
			&builder,
			"Длительность: %d мин.\n",
			*movie.RuntimeMinutes,
		)
	}

	if len(movie.Genres) > 0 {
		genres := make(
			[]string,
			0,
			len(movie.Genres),
		)

		for _, genre := range movie.Genres {
			genres = append(
				genres,
				genre.Name,
			)
		}

		fmt.Fprintf(
			&builder,
			"Жанры: %s\n",
			strings.Join(
				genres,
				", ",
			),
		)
	}

	if movie.Description != nil &&
		strings.TrimSpace(
			*movie.Description,
		) != "" {

		builder.WriteString("\n")

		builder.WriteString(
			truncate(
				*movie.Description,
				700,
			),
		)

		builder.WriteString("\n")
	}

	if movie.PosterURL != nil &&
		strings.TrimSpace(
			*movie.PosterURL,
		) != "" {

		builder.WriteString(
			"\nПостер: ",
		)

		builder.WriteString(
			*movie.PosterURL,
		)
	}

	return builder.String()
}

func displayStatus(
	status string,
) string {
	switch status {
	case "unwatched":
		return "не просмотрен"

	case "watched":
		return "просмотрен"

	default:
		return status
	}
}

func truncate(
	value string,
	maxRunes int,
) string {
	runes := []rune(value)

	if len(runes) <= maxRunes {
		return value
	}

	return string(
		runes[:maxRunes],
	) + "..."
}
