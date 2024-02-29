package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	eventserv "github.com/ceres919/simple-grpc/internal/event_server"
	eventmanager "github.com/ceres919/simple-grpc/pkg/api/protobuf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	var (
		host string
		port int
	)
	flag.StringVar(&host, "h", "localhost", "host address")
	flag.IntVar(&port, "p", 8080, "port number")
	flag.Parse()

	host_port := fmt.Sprintf("%s:%d", host, port)
	lis, err := net.Listen("tcp", host_port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	reflection.Register(s) ///убрать???
	server := eventserv.NewServerEvent()
	eventmanager.RegisterEventsServer(s, server)

	log.Printf("Listening on %s", host_port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
