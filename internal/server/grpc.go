package server

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
)

type LocalServer interface {
	Initialise() *grpc.Server
}

func RunGrpcServer(port *int, server LocalServer) {
	grpcServer := server.Initialise()
	log.Println("Starting the server ...")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	log.Printf("Server listening at %v", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
