package main

import (
	"fmt"
	// "github.com/gofiber/fiber/v2"
)

func main() {
	fmt.Println("Web Keeper.")

	connectionString := "file:storage.db?mode=rwc"
	config, err := initialize(connectionString)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Jobs:")
	for i, urlJob := range config.UrlJobs {
		fmt.Printf("%v. %v (%vs/%vs)\n", i, urlJob.Url, urlJob.QueryPeriodSeconds, urlJob.RetryPeriodSeconds)
	}

	err = queryAndSaveWebUrl("http://google.com", connectionString)
	if err != nil {
		fmt.Println(err)
		return
	}
	// app := fiber.New()

	// app.Get("/", func(c *fiber.Ctx) error {
	// 	return c.SendString("Hello, World 👋!")
	// })

	// app.Listen(":3000")
}
