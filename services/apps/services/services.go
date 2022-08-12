package services

import (
	"context"
	"github.com/coffeenights/services/apps/config"
	"github.com/coffeenights/services/apps/models"
	pb "github.com/coffeenights/services/apps/protos/apps"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	// models "github.com/coffeenights/services/apps/models"
	"github.com/dapr/go-sdk/service/common"
	"log"
)

func (s *Server) GetApplication(ctx context.Context, in *pb.GetApplicationRequest) (*pb.GetApplicationResponse, error) {
	log.Printf("Received: %v", in.GetId())
	app := models.Application{}
	s.Db.First(&app)
	return &pb.GetApplicationResponse{}, nil
}

func (s *Server) DeployApplication() {}

type PostApplicationRequest struct {
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description,omitempty"`
	AccountId   uint64 `mapstructure:"account_id"`
}

func PostApplication(ctx context.Context, e *common.TopicEvent) (retry bool, err error) {
	c := config.LoadConfig()
	db := config.GetDbConnection(c.GetDbDSN())
	data := e.Data.(map[string]interface{})

	var request PostApplicationRequest
	err = mapstructure.Decode(data, &request)
	if err != nil {
		log.Println(err)
		return false, nil
	}

	uuidWithHyphen := uuid.NewString()

	app := models.Application{
		BaseModel:   models.BaseModel{ID: uuidWithHyphen},
		Name:        request.Name,
		Description: request.Description,
		AccountId:   request.AccountId,
	}
	result := db.Create(&app)
	if result.Error != nil {
		log.Println(result.Error)
	}
	log.Printf("CREATE: %s", app)
	return false, nil
}
