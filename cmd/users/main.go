package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"

	"github.com/coffeenights/conure/cmd/users/config"
	"github.com/coffeenights/conure/cmd/users/models"
	"github.com/coffeenights/conure/cmd/users/services"
	"github.com/coffeenights/conure/internal/server"
)

func migrate() {
	c := config.LoadConfig()
	db := config.GetDbConnection(c.GetDbDSN())
	log.Println("Starting migration")
	models.Migrate(db)
	log.Println("Migration completed")
}

//var sub = &common.Subscription{
//	PubsubName: "pubsub",
//	Topic:      "post_application",
//	Route:      "/post_application",
//}

//func runsubscriber(port *int) {
//	log.Println("Starting the subscriber ...")
//	s, err := daprd.NewService(fmt.Sprintf(":%d", *port))
//	if err != nil {
//		log.Fatalf("failed to start the server: %v", err)
//	}
//	if err := s.AddTopicEventHandler(sub, cmd.PostApplication); err != nil {
//		log.Fatalf("error adding topic subscription: %v", err)
//	}
//	if err := s.Start(); err != nil && err != http.ErrServerClosed {
//		log.Fatalf("error listenning: %v", err)
//	}
//
//}

func main() {
	var (
		migrateCmd       = flag.NewFlagSet("migrate", flag.ExitOnError)
		runserverCmd     = flag.NewFlagSet("runserver", flag.ExitOnError)
		runsubscriberCmd = flag.NewFlagSet("runsubscriber", flag.ExitOnError)
		subcommand       string
	)
	portServer := runserverCmd.Int("port", 50051, "The GRPC server port")
	//portSubscriber := runsubscriberCmd.Int("port", 50001, "The subscriber service port")

	flag.Usage = func() {
		fmt.Printf("Usage: \n")
		fmt.Printf("users [cmd] [options]\n")
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
		//runsubscriber(portSubscriber)
	default:
		flag.Usage()
	}
}
