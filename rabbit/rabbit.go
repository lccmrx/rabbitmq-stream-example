package rabbit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/lccmrx/go-swiss-knife/pkg/worker"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/ha"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

type StreamClient struct {
	rabbit    *stream.Environment
	consumers []*ha.ReliableConsumer
	producers []*ha.ReliableProducer

	wp *worker.WorkerPool
}

var MessageCount atomic.Int64

func Connect() (*StreamClient, error) {
	host := os.Getenv("RABBIT_HOST")
	port, _ := strconv.Atoi(os.Getenv("RABBIT_PORT"))
	user := os.Getenv("RABBIT_USER")
	password := os.Getenv("RABBIT_PASS")

	rabbitEnv, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetHost(host).
			SetPort(port).
			SetUser(user).
			SetPassword(password),
	)
	if err != nil {
		return nil, err
	}

	return &StreamClient{
		rabbit:    rabbitEnv,
		consumers: make([]*ha.ReliableConsumer, 0),
		producers: make([]*ha.ReliableProducer, 0),
		wp:        worker.New(),
	}, nil
}

func (rmq *StreamClient) AddConsumer(streamName, consumerName string, fn func(id, message string)) error {
	storedOffset, _ := rmq.rabbit.QueryOffset(consumerName, streamName)

	slog.Info("stored offset for stream/consumer", slog.Group("data", "stream", streamName, "consumer", consumerName, "offset", storedOffset))

	var offsetSpec stream.OffsetSpecification = stream.OffsetSpecification{}.First()
	// if storedOffset > 0 {
	// 	slog.Info("Resuming from stored offset", "offset", storedOffset)
	// 	offsetSpec = stream.OffsetSpecification{}.Next()
	// }

	opts := stream.NewConsumerOptions().
		SetConsumerName(consumerName).
		SetClientProvidedName(consumerName).
		SetOffset(offsetSpec).
		SetInitialCredits(10).
		SetAutoCommit(nil)

	consumer, err := ha.NewReliableConsumer(rmq.rabbit, streamName, opts, func(ctx stream.ConsumerContext, message *amqp.Message) {
		id := message.Properties.MessageID
		data := string(message.GetData())

		rmq.wp.Dispatch(func() error {
			fn(id.(string), data)
			return nil
		})
	})
	rmq.consumers = append(rmq.consumers, consumer)
	return err
}

func (rmq *StreamClient) AddProducer(streamName, producerName string, msgCh <-chan []byte) error {
	// Declare the stream, Set segmentsize and Maxbytes on stream
	err := rmq.rabbit.DeclareStream(os.Getenv("RABBIT_STREAM"), stream.NewStreamOptions().
		SetMaxSegmentSizeBytes(stream.ByteCapacity{}.MB(1)).
		SetMaxLengthBytes(stream.ByteCapacity{}.MB(2)),
	)

	if err != nil {
		return err
	}

	opts := stream.NewProducerOptions().
		SetProducerName(producerName)

	producer, err := ha.NewReliableProducer(rmq.rabbit, streamName, opts, func(messageConfirm []*stream.ConfirmationStatus) {
		for _, msg := range messageConfirm {
			if !msg.IsConfirmed() {
				slog.Error("message failed", slog.Any("data", msg.GetMessage().GetData()))
				return
			}
			MessageCount.Add(1)
			slog.Info("message confirmed", slog.Any("message-id", msg.GetMessage().GetMessageProperties().MessageID))
		}
	})
	if err != nil {
		return err
	}

	go func() {
		for rawData := range msgCh {
			var data map[string]any
			json.Unmarshal(rawData, &data)
			msg := amqp.NewMessage(rawData)

			msg.Properties = &amqp.MessageProperties{
				MessageID: fmt.Sprintf("%.0f-%s", data["test"].(float64), uuid.NewString()),
			}
			err := producer.Send(msg)
			if err != nil {
				slog.Error("failed to produce message")
			}
		}
	}()

	rmq.producers = append(rmq.producers, producer)

	return nil
}

func (rmq *StreamClient) Close() {
	for _, producer := range rmq.producers {
		producer.Close()
	}

	for _, consumer := range rmq.consumers {
		consumer.Close()
	}

	rmq.wp.Wait()

	rmq.rabbit.Close()
}
