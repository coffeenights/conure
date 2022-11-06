package main

import (
	"github.com/coffeenights/conure/api/routes"
	"log"
)

func main() {
	r := routes.GenerateRouter()
	err := r.Run()
	if err != nil {
		log.Fatal(err)
	}
}
