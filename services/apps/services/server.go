package services

import (
	"github.com/coffeenights/services/apps/config"
	pb "github.com/coffeenights/services/apps/protos/apps"
	"gorm.io/gorm"
)

// Server ApplicationServiceServer object
type Server struct {
	Config *config.Config
	Db     *gorm.DB
	pb.ApplicationServiceServer
}
