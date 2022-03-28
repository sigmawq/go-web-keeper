package main

import (
	"fmt"
	// "github.com/gofiber/fiber/v2"
)

// TODO
// 1. Image loading
// 2. Scripts
// 3. HTTP server to serve thing
// 4. Database for scripts and image loading

func main() {
	fmt.Println("Web Keeper.")

	connectionString := "file:storage.db?mode=rwc"
	config, dbContext, err := initialize(connectionString)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Jobs:")
	for i, urlJob := range config.UrlJobs {
		fmt.Printf("%v. %v (%vs/%vs)\n", i, urlJob.Url, urlJob.QueryPeriodSeconds, urlJob.RetryPeriodSeconds)
	}

	err = queryAndSaveWebUrl("http://google.com", &dbContext)
	if err != nil {
		fmt.Println(err)
		return
	}

	for i, urlJob := range config.UrlJobs {
		go queryAndStoreRoutine(i, urlJob, &dbContext)
	}

	select {}
	// app := fiber.New()

	// app.Get("/", func(c *fiber.Ctx) error {
	// 	return c.SendString("Hello, World 👋!")
	// })

	// app.Listen(":3000")
}
