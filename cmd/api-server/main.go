package main

import (
	"github.com/coffeenights/conure/cmd/api-server/routes"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"log"
)

func main() {
	r := routes.GenerateRouter()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	err := r.Run(":8080")
	if err != nil {
		log.Fatal(err)
	}
}
