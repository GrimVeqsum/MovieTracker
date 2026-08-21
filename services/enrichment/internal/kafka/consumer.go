package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"movie-platform/enrichment/internal/events"
	"movie-platform/enrichment/internal/library"
	"movie-platform/enrichment/internal/poiskkino"

	"github.com/twmb/franz-go/pkg/kgo"
)

type MovieProvider interface {
	FindMovie(
		ctx context.Context,
		title string,
		releaseYear *int,
	) (*poiskkino.MovieDetails, error)
}

type LibraryClient interface {
	UpdateMetadata(
		ctx context.Context,
		movieID string,
		request library.UpdateMetadataRequest,
	) error
}

type Consumer struct {
	client        *kgo.Client
	movieProvider MovieProvider
	libraryClient LibraryClient
}

func NewConsumer(
	broker string,
	topic string,
	group string,
	movieProvider MovieProvider,
	libraryClient LibraryClient,
) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(broker),

		kgo.ConsumerGroup(group),

		kgo.ConsumeTopics(topic),

		kgo.ConsumeResetOffset(
			kgo.NewOffset().AtStart(),
		),

		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client:        client,
		movieProvider: movieProvider,
		libraryClient: libraryClient,
	}, nil
}

func (consumer *Consumer) Run(
	ctx context.Context,
) {
	for {
		if ctx.Err() != nil {
			return
		}

		fetches := consumer.client.PollRecords(
			ctx,
			1,
		)

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, err := range errs {
				log.Printf(
					"Kafka poll error: %v",
					err,
				)
			}

			continue
		}

		for _, record := range fetches.Records() {
			if err := consumer.process(
				ctx,
				record,
			); err != nil {
				log.Printf(
					"event processing failed: %v",
					err,
				)

				time.Sleep(2 * time.Second)

				continue
			}

			if err := consumer.client.CommitRecords(
				ctx,
				record,
			); err != nil {
				log.Printf(
					"Kafka commit error: %v",
					err,
				)
			}
		}
	}
}

func (consumer *Consumer) process(
	ctx context.Context,
	record *kgo.Record,
) error {
	var event events.MovieEvent

	if err := json.Unmarshal(
		record.Value,
		&event,
	); err != nil {
		log.Printf(
			"invalid Kafka event: %s",
			string(record.Value),
		)

		return nil
	}

	// Enrichment обрабатывает только создание фильма.
	if event.Type != "MovieCreated" {
		return nil
	}

	if strings.TrimSpace(event.Title) == "" {
		log.Printf(
			"MovieCreated has empty title: movie_id=%s event_id=%s",
			event.MovieID,
			event.EventID,
		)

		return nil
	}

	log.Printf(
		"MovieCreated received: movie_id=%s title=%q",
		event.MovieID,
		event.Title,
	)

	movie, err := consumer.movieProvider.FindMovie(
		ctx,
		event.Title,
		event.ReleaseYear,
	)
	if err != nil {
		if errors.Is(
			err,
			poiskkino.ErrMovieNotFound,
		) {
			log.Printf(
				"movie not found: title=%q",
				event.Title,
			)

			return nil
		}

		return err
	}

	log.Printf(
		"movie found: id=%d name=%q year=%d",
		movie.ID,
		movie.Name,
		movie.Year,
	)

	if movie.MovieLength != nil {
		log.Printf(
			"runtime: %d minutes",
			*movie.MovieLength,
		)
	}

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

	err = consumer.libraryClient.UpdateMetadata(
		ctx,
		event.MovieID,
		library.UpdateMetadataRequest{
			UserID:           event.UserID,
			ExternalID:       strconv.Itoa(movie.ID),
			MetadataProvider: "poiskkino",
			OriginalTitle:    movie.AlternativeName,
			Description:      movie.Description,
			ReleaseYear:      movie.Year,
			PosterURL:        movie.Poster.URL,
			RuntimeMinutes:   movie.MovieLength,
			Genres:           genres,
		},
	)
	if err != nil {
		return err
	}

	log.Printf(
		"movie metadata saved: movie_id=%s",
		event.MovieID,
	)

	return nil
}

func (consumer *Consumer) Close() {
	consumer.client.Close()
}
