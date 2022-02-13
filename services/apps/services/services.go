package services

import (
	"context"
	pb "github.com/coffeenights/services/apps/protos/apps"
	"log"
)

func (s *Server) GetApplication(ctx context.Context, in *pb.GetApplicationRequest) (*pb.GetApplicationResponse, error) {
	log.Printf("Received: %v", in.GetId())
	return &pb.GetApplicationResponse{}, nil
}
