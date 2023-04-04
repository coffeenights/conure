package services

import (
	"github.com/coffeenights/conure/cmd/apps/config"
	pb "github.com/coffeenights/conure/cmd/apps/protos/apps"
	"gorm.io/gorm"
)

// Server ApplicationServiceServer object
type Server struct {
	Config *config.Config
	Db     *gorm.DB
	pb.ApplicationServiceServer
}
