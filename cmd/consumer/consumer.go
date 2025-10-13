package main

import (
	"log"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"rabbit-heartbeat/rabbit"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
}

func main() {
	slog.Info("starting up")
	slog.Info("connecting to rabbit")
	client, err := rabbit.Connect()
	if err != nil {
		log.Fatal(err)
	}

	slog.Info("adding consumer")
	err = client.AddConsumer(os.Getenv("RABBIT_STREAM"), "consumer", consumer)
	if err != nil {
		log.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	client.Close()

	slog.Info("shutting down")
}

func consumer(id, message string) {
	slog.Info("received rabbit message",
		slog.Any("message-id", id),
		slog.String("data", message),
	)

	time.Sleep(time.Duration(rand.Intn(10)) * time.Second)
}
