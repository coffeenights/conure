package main

import (
	"github.com/coffeenights/conure/cmd/api-server/routes"
	_ "github.com/joho/godotenv/autoload"
	"log"
)

func main() {
	r := routes.GenerateRouter()
	err := r.Run(":8080")
	if err != nil {
		log.Fatal(err)
	}
}
