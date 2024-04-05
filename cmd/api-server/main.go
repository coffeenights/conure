package main

import (
	apiConfig "github.com/coffeenights/conure/cmd/api-server/config"
	"github.com/coffeenights/conure/cmd/api-server/routes"
	"github.com/coffeenights/conure/internal/config"
	_ "github.com/joho/godotenv/autoload"
	"log"
)

func main() {
	r := routes.GenerateRouter()
	conf := config.LoadConfig(apiConfig.Config{})

	err := r.Run(conf.APIHost + ":8080")
	if err != nil {
		log.Fatal(err)
	}
}
