package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"sync"

	eventclient "github.com/ceres919/simple-grpc/internal/event_client"
	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
	"google.golang.org/grpc"
)

func main() {
	var (
		destination string
		port        string
		sender_id   string
	)

	var wg sync.WaitGroup
	wg.Add(1)

	flag.StringVar(&destination, "dst", "localhost", "destination address")
	flag.StringVar(&port, "p", "8080", "port number")
	flag.StringVar(&sender_id, "sender-id", "0", "sender id")
	flag.Parse()

	destination_port := fmt.Sprintf("%s:%s", destination, port)
	conn, err := grpc.Dial(destination_port, grpc.WithInsecure()) ///todo
	if err != nil {
		log.Fatal(err)
	}
	client := eventmanager.NewEventsClient(conn)
	go func() {
		defer wg.Done()
		int_sender_id, _ := strconv.ParseInt(sender_id, 10, 64)
		eventclient.RunEventsClient(int_sender_id, client)
	}()
	defer conn.Close()

	wg.Wait()
}
