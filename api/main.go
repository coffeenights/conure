package main

import (
	"github.com/gofiber/fiber/v2"
	"log"
)

func main() {
	app := fiber.New()

	spaces := app.Group("spaces")
	handler := func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
	alphaV1 := spaces.Group("/alphav1")
	alphaV1.Get("/list", handler)

	log.Fatal(app.Listen(":3000"))
}
