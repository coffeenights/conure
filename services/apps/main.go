package main

import (
	"flag"
	"fmt"
	"github.com/coffeenights/services/apps/config"
	"github.com/coffeenights/services/apps/models"
	pb "github.com/coffeenights/services/apps/protos/apps"
	"github.com/coffeenights/services/apps/services"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"net"
	"os"
)

func getDbConnection(dsn string) *gorm.DB {
	log.Println("Connecting to the database ...")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to the database: %s", err)
	}
	return db
}

func runServer(port *int) {
	server := services.Server{Config: config.LoadConfig()}

	// Database connection
	server.Db = getDbConnection(server.Config.GetDbDSN())

	// Start GRPC Server
	log.Println("Starting the server ...")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterApplicationServiceServer(s, &server)
	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func migrate() {
	c := config.LoadConfig()
	db := getDbConnection(c.GetDbDSN())
	log.Println("Starting migration")
	models.Migrate(db)
	log.Println("Migration completed")
}

func main() {
	var (
		migrateCmd   = flag.NewFlagSet("migrate", flag.ExitOnError)
		runserverCmd = flag.NewFlagSet("runserver", flag.ExitOnError)
		subcommand   string
	)
	port := runserverCmd.Int("port", 50051, "The GRPC server port")

	flag.Usage = func() {
		fmt.Printf("Usage: \n")
		fmt.Printf("apps [cmd] [options]\n")
		fmt.Printf("  Commands available:\n")
		fmt.Printf("\tmigrate     Run database migrations\n")
		fmt.Printf("\trunserver   Run GRPC server\n")
	}
	if len(os.Args) >= 2 {
		subcommand = os.Args[1]
	}

	switch subcommand {
	case "runserver":
		err := runserverCmd.Parse(os.Args[2:])
		if err != nil {
			panic(err)
		}
		runServer(port)
	case "migrate":
		_ = migrateCmd.Parse(os.Args[2:])
		migrate()
	default:
		flag.Usage()
	}

}
