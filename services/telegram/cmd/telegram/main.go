package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"movie-platform/telegram/internal/backend"
	"movie-platform/telegram/internal/bot"
	"movie-platform/telegram/internal/config"
	"movie-platform/telegram/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf(
			"config error: %v",
			err,
		)
		return
	}

	telegramClient :=
		telegram.NewClient(
			cfg.BotToken,
		)

	backendClient :=
		backend.NewClient(
			cfg.AuthURL,
			cfg.LibraryURL,
		)

	ctx, stop :=
		signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
	defer stop()

	me, err :=
		telegramClient.GetMe(ctx)

	if err != nil {
		log.Printf(
			"Telegram getMe error: %v",
			err,
		)
		return
	}

	log.Printf(
		"telegram bot connected: id=%d username=@%s",
		me.ID,
		me.Username,
	)

	telegramBot :=
		bot.New(
			telegramClient,
			backendClient,
		)

	log.Println(
		"telegram-service started",
	)

	if err :=
		telegramBot.Run(ctx); err != nil {

		log.Printf(
			"telegram bot stopped with error: %v",
			err,
		)
	}

	log.Println(
		"telegram-service stopped",
	)
}
