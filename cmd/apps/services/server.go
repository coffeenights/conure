package services

import (
	apps_config "github.com/coffeenights/conure/cmd/apps/config"
	pb "github.com/coffeenights/conure/cmd/apps/protos/apps"
	"github.com/coffeenights/conure/internal/config"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// Server ApplicationServiceServer object
type Server struct {
	Config *apps_config.Config
	Db     *gorm.DB
	pb.ApplicationServiceServer
}

func (s *Server) Initialise() *grpc.Server {
	s.Config = config.LoadConfig(apps_config.Config{})
	dsn := config.GetDbDSN(s.Config.DbUrl)
	s.Db = config.GetDbConnection(dsn)
	grpcServer := grpc.NewServer()
	pb.RegisterApplicationServiceServer(grpcServer, s)
	return grpcServer
}
