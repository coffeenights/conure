package services

import (
	"github.com/coffeenights/conure/services/apps/config"
	pb "github.com/coffeenights/conure/services/apps/protos/apps"
	"gorm.io/gorm"
)

// Server ApplicationServiceServer object
type Server struct {
	Config *config.Config
	Db     *gorm.DB
	pb.ApplicationServiceServer
}
