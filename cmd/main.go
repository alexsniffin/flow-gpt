package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	fsm2 "flow-gpt/internal/fsm"
	"flow-gpt/internal/logger"
	zLog "github.com/rs/zerolog/log"
)

const (
	LogLevel = "debug"
)

func main() {
	problemFlag := flag.String("problem", "", "problem")
	flag.Parse()

	log.Println("starting application...")
	err := logger.NewGlobal(LogLevel, false)
	if err != nil {
		log.Panicf("failed to initialize logger: %v", err)
	}

	fsm, err := fsm2.New(*problemFlag, 0)
	if err != nil {
		zLog.Fatal().Err(err).Msg("failed to initialize FSM")
	}

	http.HandleFunc("/ws", fsm.Handler)
	go func() {
		zLog.Fatal().Err(http.ListenAndServe(":8080", nil)).Msg("failed to start server")
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		fsm.Process(ctx)
	}()

	<-ctx.Done()
	err = fsm.Browser.Close()
	if err != nil {
		zLog.Error().Msgf("failed to close browser: %v", err)
	}

	zLog.Info().Msg("shutting down application")
}
