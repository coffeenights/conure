package main

import (
	"flag"
	"fmt"
	apps_config "github.com/coffeenights/conure/cmd/apps/config"
	"github.com/coffeenights/conure/cmd/apps/models"
	"github.com/coffeenights/conure/cmd/apps/services"
	"github.com/coffeenights/conure/internal/config"
	"github.com/coffeenights/conure/internal/server"
	"github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/grpc"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"net/http"
	"os"
)

func migrate() {
	c := config.LoadConfig(apps_config.Config{})
	dsn := config.GetDbDSN(c.DbUrl)
	db := config.GetDbConnection(dsn)
	log.Println("Starting migration")
	models.Migrate(db)
	log.Println("Migration completed")
}

func runsubscriber(port *int) {
	log.Println("Starting the subscriber ...")
	var sub = &common.Subscription{
		PubsubName: "pubsub",
		Topic:      "post_application",
		Route:      "/post_application",
	}

	s, err := daprd.NewService(fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to start the server: %v", err)
	}
	if err := s.AddTopicEventHandler(sub, services.PostApplication); err != nil {
		log.Fatalf("error adding topic subscription: %v", err)
	}
	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error listenning: %v", err)
	}

}

func main() {
	var (
		migrateCmd       = flag.NewFlagSet("migrate", flag.ExitOnError)
		runserverCmd     = flag.NewFlagSet("runserver", flag.ExitOnError)
		runsubscriberCmd = flag.NewFlagSet("runsubscriber", flag.ExitOnError)
		subcommand       string
	)
	portServer := runserverCmd.Int("port", 50051, "The GRPC server port")
	portSubscriber := runsubscriberCmd.Int("port", 50001, "The subscriber service port")

	flag.Usage = func() {
		fmt.Printf("Usage: \n")
		fmt.Printf("apps [cmd] [options]\n")
		fmt.Printf("  Commands available:\n")
		fmt.Printf("\tmigrate     	Run database migrations\n")
		fmt.Printf("\trunserver   	Run GRPC server\n")
		fmt.Printf("\trunsubscriber  Run pub/sub subscriber\n")
	}
	if len(os.Args) >= 2 {
		subcommand = os.Args[1]
	}

	switch subcommand {
	case "runserver":
		err := runserverCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("failed to start the server: %v", err)
		}
		server.RunGrpcServer(portServer, &services.Server{})
	case "migrate":
		_ = migrateCmd.Parse(os.Args[2:])
		migrate()
	case "runsubscriber":
		err := runsubscriberCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatalf("failed to start the service: %v", err)
		}
		runsubscriber(portSubscriber)
	default:
		flag.Usage()
	}
}
