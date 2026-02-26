package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"vn.io.arda/notification/internal/application"
	"vn.io.arda/notification/internal/config"
	"vn.io.arda/notification/internal/infrastructure/keycloak"
	"vn.io.arda/notification/internal/infrastructure/postgres"
	kafkaconsumer "vn.io.arda/notification/internal/kafka"
	transporthttp "vn.io.arda/notification/internal/transport/http"
)

func main() {
	// ── Logging ──────────────────────────────────────────────────────────────
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	// ── Config ───────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	if cfg.Server.Env == "production" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	log.Info().Str("env", cfg.Server.Env).Str("port", cfg.Server.Port).Msg("starting arda-notification")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Database ──────────────────────────────────────────────────────────────
	dsn := "host=" + cfg.Database.Host +
		" port=" + strconv.Itoa(cfg.Database.Port) +
		" dbname=" + cfg.Database.Name +
		" user=" + cfg.Database.User +
		" password=" + cfg.Database.Password +
		" sslmode=disable"

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("postgres ping failed")
	}
	log.Info().Msg("postgres connected")

	// ── Repository & SSE Hub ─────────────────────────────────────────────────
	repo := postgres.New(pool)
	hub := transporthttp.NewHub()

	// ── IAM Resolver (Keycloak Admin API) ─────────────────────────────────────
	iamResolver := keycloak.New(
		cfg.Keycloak.BaseURL,
		cfg.Keycloak.AdminRealm,
		cfg.Keycloak.AdminClientID,
		cfg.Keycloak.AdminClientSecret,
	)

	// ── Application Service ───────────────────────────────────────────────────
	svc := application.NewService(repo, hub, iamResolver)

	// ── HTTP Server ───────────────────────────────────────────────────────────
	handler := transporthttp.NewHandler(svc, hub)
	router := transporthttp.NewRouter(handler, cfg.Keycloak.BaseURL)

	// ── Kafka Consumer ────────────────────────────────────────────────────────
	consumer, err := kafkaconsumer.New(
		cfg.Kafka.Brokers,
		cfg.Kafka.ConsumerGroupID,
		cfg.Kafka.Topics,
		svc,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create kafka consumer")
	}

	// Start Kafka consumer in background
	go consumer.Start(ctx)
	log.Info().Strs("topics", cfg.Kafka.Topics).Msg("kafka consumer started")

	// ── TTL Purge Job (every 24h) ─────────────────────────────────────────────
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				svc.PurgeTTL(context.Background(), cfg.TTL.RetentionDays)
			case <-ctx.Done():
				return
			}
		}
	}()

	// ── Start HTTP Server ─────────────────────────────────────────────────────
	go func() {
		log.Info().Str("port", cfg.Server.Port).Msg("HTTP server listening")
		if err := router.Start(":" + cfg.Server.Port); err != nil {
			log.Info().Msg("HTTP server stopped")
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	<-ctx.Done()
	log.Info().Msg("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := router.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	log.Info().Msg("arda-notification stopped")
}
