package services

import (
	"context"
	apps_config "github.com/coffeenights/conure/cmd/apps/config"
	"github.com/coffeenights/conure/cmd/apps/models"
	pb "github.com/coffeenights/conure/cmd/apps/protos/apps"
	"github.com/coffeenights/conure/internal/config"
	"github.com/dapr/go-sdk/service/common"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"log"
)

func (s *Server) GetApplication(ctx context.Context, in *pb.GetApplicationRequest) (*pb.GetApplicationResponse, error) {
	log.Printf("Received: %v", in.GetId())
	app := models.Application{}
	s.Db.First(&app)
	return &pb.GetApplicationResponse{}, nil
}

func (s *Server) ListApplications(ctx context.Context, in *pb.ListApplicationsRequest) (*pb.ListApplicationsResponse, error) {
	log.Printf("Received: %v", in.GetAccountId())
	app := models.Application{AccountId: in.GetAccountId()}
	var apps []models.Application
	s.Db.Model(app).Find(&apps)
	var applicationsResult []*pb.Application
	for _, applicationEntry := range apps {
		application := &pb.Application{
			Id:          applicationEntry.ID,
			Name:        applicationEntry.Name,
			Description: applicationEntry.Description,
			Clusters:    nil,
			Created:     applicationEntry.CreatedAt.String(),
			Modified:    applicationEntry.UpdatedAt.String(),
			AccountId:   applicationEntry.AccountId,
			Active:      true,
		}
		applicationsResult = append(applicationsResult, application)
	}
	return &pb.ListApplicationsResponse{
		Applications: applicationsResult,
	}, nil
}

func (s *Server) DeployApplication() {}

type PostApplicationRequest struct {
	Name          string `mapstructure:"name"`
	Description   string `mapstructure:"description,omitempty"`
	AccountId     uint64 `mapstructure:"account_id"`
	EnvironmentId string `mapstructure:"environment_id"`
}

func PostApplication(ctx context.Context, e *common.TopicEvent) (retry bool, err error) {
	c := config.LoadConfig(apps_config.Config{})
	dsn := config.GetDbDSN(c.DbUrl)
	db := config.GetDbConnection(dsn)
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
