package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/coffeenights/conure/cmd/api-server/auth"
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/database"
	"github.com/coffeenights/conure/cmd/api-server/routes"
	"github.com/coffeenights/conure/cmd/api-server/variables"
	"github.com/coffeenights/conure/internal/config"
	_ "github.com/joho/godotenv/autoload"
)

const (
	SystemNamespace = "conure-system"
)

func runServer(address string, port int) {
	r := routes.GenerateRouter()
	log.Println("Running the server...")
	err := r.Run(fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		log.Fatal(err)
	}
}

func createSuperUser(email string) {
	conf := config.LoadConfig(apiConfig.Config{})
	log.Println("Connecting to MongoDB")
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	if err != nil {
		log.Panic(err)
	}
	log.Println("Connected to MongoDB")
	auth.CreateSuperuser(mongo, email)
}

func createSecretKey() {
	_ = config.LoadConfig(apiConfig.Config{})
	log.Println("Creating secret key...")
	keyStore := variables.NewK8sSecretKey(SystemNamespace)
	err := keyStore.Generate()
	if err != nil {
		log.Panic(err)
	}
	log.Println("Secret key created")
}

func resetSuperUserPassword(email string) {
	conf := config.LoadConfig(apiConfig.Config{})
	log.Println("Connecting to MongoDB")
	mongo, err := database.ConnectToMongoDB(conf.MongoDBURI, conf.MongoDBName)
	if err != nil {
		log.Panic(err)
	}
	log.Println("Connected to MongoDB")
	auth.ResetSuperuserPassword(mongo, email)
}

func main() {
	var (
		runserverCmd              = flag.NewFlagSet("runserver", flag.ExitOnError)
		createsuperuserCmd        = flag.NewFlagSet("createsuperuser", flag.ExitOnError)
		resetSuperUserPasswordCmd = flag.NewFlagSet("resetsuperuserpassword", flag.ExitOnError)
		subcommand                string
	)

	addressServer := runserverCmd.String("address", "localhost", "The HTTP server bind address.")
	portServer := runserverCmd.Int("port", 8080, "The HTTP server port")
	emailSuperuser := createsuperuserCmd.String("email", "", "The email of the superuser")
	emailSuperuserReset := resetSuperUserPasswordCmd.String("email", "", "The email of the superuser")

	flag.Usage = func() {
		fmt.Printf("Usage: \n")
		fmt.Printf("conure_api [cmd] [options]\n")
		fmt.Printf("  Commands available:\n")
		fmt.Printf("\trunserver        Run the HTTP server\n")
		fmt.Printf("\tcreatesuperuser  Create the super user for your account\n")
		fmt.Printf("\tresetsuperuserpassword  Reset the super user password\n")
		fmt.Printf("\tcreatesecretkey  Create the secret key for your account\n")
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
		runServer(*addressServer, *portServer)
	case "createsuperuser":
		err := createsuperuserCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatal(err)
		}
		if *emailSuperuser == "" {
			fmt.Println("Error: Missing -email flag")
			createsuperuserCmd.Usage()
			os.Exit(1)
		}
		createSuperUser(*emailSuperuser)
	case "createsecretkey":
		createSecretKey()
	case "resetsuperuserpassword":
		err := resetSuperUserPasswordCmd.Parse(os.Args[2:])
		if err != nil {
			log.Fatal(err)
		}
		if *emailSuperuserReset == "" {
			fmt.Println("Error: Missing -email flag")
			resetSuperUserPasswordCmd.Usage()
			os.Exit(1)
		}
		resetSuperUserPassword(*emailSuperuserReset)
	default:
		flag.Usage()
	}
}
