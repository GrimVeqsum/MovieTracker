package main

import (
	"context"
	"fmt"
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
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err :=
		config.Load()

	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	telegramClient :=
		telegram.NewClient(
			cfg.BotToken,
		)

	backendClient :=
		backend.NewClient(
			cfg.AuthURL,
			cfg.LibraryURL,
			cfg.AuthServiceSecret,
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
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("Telegram getMe: %w", err)
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

	if err := telegramBot.Run(ctx); err != nil {
		return fmt.Errorf("telegram bot: %w", err)
	}
	log.Println("telegram-service stopped")
	return nil
}
