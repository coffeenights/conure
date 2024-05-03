package main

import (
	"flag"
	"fmt"
	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/routes"
	"github.com/coffeenights/conure/internal/config"
	_ "github.com/joho/godotenv/autoload"
	"log"
	"os"
)

func runserver(address string, port int) {
	r := routes.GenerateRouter()
	err := r.Run(fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		log.Fatal(err)
	}
}

func createsuperuser(email string) {
	conf := config.LoadConfig(apiConfig.Config{})
	log.Println("Connecting to MongoDB")
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	if err != nil {
		log.Panic(err)
	}
	log.Println("Connected to MongoDB")
	auth.CreateSuperuser(mongo, email)
}

func main() {
	var (
		runserverCmd       = flag.NewFlagSet("runserver", flag.ExitOnError)
		createsuperuserCmd = flag.NewFlagSet("createsuperuser", flag.ExitOnError)
		subcommand         string
	)
	addressServer := runserverCmd.String("address", "localhost", "The HTTP server bind address.")
	portServer := runserverCmd.Int("port", 8080, "The HTTP server port")

	emailSuperuser := createsuperuserCmd.String("email", "", "The email of the superuser")

	flag.Usage = func() {
		fmt.Printf("Usage: \n")
		fmt.Printf("conure_api [cmd] [options]\n")
		fmt.Printf("  Commands available:\n")
		fmt.Printf("\trunserver        Run the HTTP server\n")
		fmt.Printf("\tcreatesuperuser  Create the super user for your account\n")
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
		runserver(*addressServer, *portServer)
	case "createsuperuser":
		if *emailSuperuser == "" {
			log.Printf("Error: email is required for createsuperuser command")
			createsuperuserCmd.Usage()
			os.Exit(1)
		}
		createsuperuser(*emailSuperuser)
	default:
		flag.Usage()
	}
}
