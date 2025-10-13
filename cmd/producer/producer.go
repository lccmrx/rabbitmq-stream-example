package main

import (
	"encoding/json"
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

	ch := make(chan []byte, 999)
	go func() {
		for i := 0; ; i++ {
			time.Sleep(time.Duration(rand.Intn(3)) * time.Second)
			dataMap := map[string]any{
				"test": i,
			}

			data, _ := json.Marshal(dataMap)
			ch <- data
		}
	}()

	slog.Info("adding producer")
	err = client.AddProducer(os.Getenv("RABBIT_STREAM"), "producer", ch)
	if err != nil {
		log.Fatal(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	client.Close()
	slog.Info("shutting down", slog.Int64("sent", rabbit.MessageCount.Load()))
}
