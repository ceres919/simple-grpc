package eventclient

import (
	"fmt"
	"time"

	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"
)

func notifyer(event *eventmanager.EventResponse, routingKey string, queueName string) {
	const (
		exchange    = "event.ex"
		reconnDelay = 5
	)
	var (
		conn *amqp.Connection
		ch   *amqp.Channel
		q    amqp.Queue
	)
	defer conn.Close()
	defer ch.Close()

	rechannel := func() {
		var err error
		ch, err = conn.Channel()
		if err != nil {
			fmt.Printf("Unable to open a channel. Error: %s\n> ", err)
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
			fmt.Printf("Unable to declare exchange. Error: %s\n> ", err)
		}
		q, err = ch.QueueDeclare(
			queueName, // queue name
			true,      // durable
			false,     // delete when unused
			false,     // exclusive
			false,     // no-wait
			nil,       // arguments
		)
		if err != nil {
			fmt.Printf("Unable to declare queue. Error: %s\n> ", err)
		}

		if err := ch.QueueBind(
			q.Name,     // queue name
			routingKey, // routing key
			exchange,   // exchange
			false,      // no-wait
			nil,        // arguments
		); err != nil {
			fmt.Printf("Unable to bind queue. Error: %s\n> ", err)
		}
	}

	redial := func() error {
		var err error
		conn, err = amqp.Dial("amqp://guest:guest@localhost:5672/")
		if err != nil {
			return err
		}
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
			_, ok := <-conn.NotifyClose(make(chan *amqp.Error))
			if !ok {
				break
			}
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
			_, ok := <-ch.NotifyClose(make(chan *amqp.Error))
			if !ok || ch.IsClosed() {
				ch.Close() // close again, ensure closed flag set when connection closed
				break
			}
			for {
				time.Sleep(reconnDelay * time.Second)
				rechannel()
			}
		}
	}()

	messagesChan := make(chan amqp.Delivery)

	go func() {
		for {
			messages, err := ch.Consume(
				q.Name, // queue name
				"",     // consumer
				false,  // auto ack
				false,  // exclusive
				false,  // no-local
				false,  // no-wait
				nil,    // arguments
			)
			if err != nil {
				fmt.Printf("Unable to consume notifications after delay (%d)s.\n> ", reconnDelay)
				time.Sleep(reconnDelay * time.Second)
				continue
			}
			for msg := range messages {
				messagesChan <- msg
			}
			time.Sleep(reconnDelay * time.Second)
			if ch.IsClosed() {
				fmt.Printf("Connection is lost. Reconnecting...\n> ")
			}
		}
	}()

	var forever chan struct{}

	go func() {
		for message := range messagesChan {
			err := proto.Unmarshal(message.Body, event)
			if err != nil {
				panic(err)
			}
			fmt.Println("Notification!")
			t := time.UnixMilli(event.Time).Local().Format(time.DateTime)
			eId, _ := uuid.FromBytes(event.EventId)
			fmt.Printf(
				"Event {\n  senderId: %d\n  eventId: %s\n  time: %s\n  name: '%s'\n}\n> ",
				event.SenderId,
				eId.String(),
				t,
				event.Name,
			)
		}
	}()

	<-forever
}
