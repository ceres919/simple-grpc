package eventserver

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/ceres919/simple-grpc/internal/models"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"
)

func publish(publisherChannel chan models.Event) {
	const (
		exchange    = "event.ex"
		reconnDelay = 5
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		var (
			conn *amqp.Connection
			ch   *amqp.Channel
		)
		defer conn.Close()
		defer ch.Close()

		rechannel := func() {
			var err error
			ch, err = conn.Channel()
			if err != nil {
				log.Printf("Unable to open a channel. Error: %s", err)
			}
			if err := ch.ExchangeDeclare(
				exchange, // exchange name
				"direct", // type
				true,     // durable
				false,    // delete when unused
				false,    // exclusive
				false,    // no-wait
				nil,      // arguments
			); err != nil {
				log.Printf("Unable to declare exchange. Error: %s", err)
			}
		}

		redial := func() error {
			var err error
			conn, err = amqp.Dial("amqp://guest:guest@localhost:5672/")
			if err != nil {
				log.Println("Unable to connect to RabbitMQ")
				return err
			}
			log.Println("Connected to RabbitMQ")
			defer rechannel()
			return nil
		}

		for {
			if err := redial(); err == nil {
				break
			}
			time.Sleep(reconnDelay * time.Second)
		}

		go func() {
			for {
				reason, ok := <-conn.NotifyClose(make(chan *amqp.Error))
				if !ok {
					log.Println("Connection is closed gracefully or closed by devs. Won't reconnect")
					break
				}
				log.Printf("Will reconnect because connection closed with reason: %v\n", reason)
				for {
					time.Sleep(reconnDelay * time.Second)
					if err := redial(); err == nil {
						break
					}
				}
			}
		}()

		go func() {
			for {
				reason, ok := <-ch.NotifyClose(make(chan *amqp.Error))
				if !ok || ch.IsClosed() {
					log.Println("Channel is closed gracefully or closed by devs. Won't reconnect")
					ch.Close() // close again, ensure closed flag set when connection closed
					break
				}
				log.Printf("Will reconnect because channel is closed with reason: %v\n", reason)
				for {
					time.Sleep(reconnDelay * time.Second)
					rechannel()
				}
			}
		}()

		for {
			event := <-publisherChannel

			body, err := proto.Marshal(ArchiveEvent(event))
			if err != nil {
				log.Fatalf("Unable to marshal event. Error: %s", err)
			}

			go func() {
				for {
					if err := ch.PublishWithContext(
						ctx,
						exchange,
						strconv.Itoa(int(event.SenderId)),
						false,
						false,
						amqp.Publishing{Body: body},
					); err == nil {
						log.Println("[x] Sent:", body)
						break
					} else {
						log.Printf("Unable to publish event. Error: %s\n", err)
					}
					time.Sleep(reconnDelay * time.Second)
				}
			}()
		}
	}()
}
