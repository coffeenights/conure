package services

import (
	"context"
	pb "github.com/coffeenights/services/apps/protos/apps"
	"github.com/dapr/go-sdk/service/common"
	"log"
)

func (s *Server) GetApplication(ctx context.Context, in *pb.GetApplicationRequest) (*pb.GetApplicationResponse, error) {
	log.Printf("Received: %v", in.GetId())
	return &pb.GetApplicationResponse{}, nil
}

type PostApplicationRequest struct {
	Name        string  `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	AccountId   uint64  `json:"account_id,omitempty"`
	Active      bool    `json:"active,omitempty"`
}

func PostApplication(ctx context.Context, e *common.TopicEvent) (retry bool, err error) {
	log.Printf("Subscriber received: %s", e.Data)
	return false, nil
}
