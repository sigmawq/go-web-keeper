package main

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"log"
	"os"
	"time"
	"strings"
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

	app := fiber.New()

	app.Get("/data/:protocol/:domain/:from/:to", func(c *fiber.Ctx) error {
		log.Println("Request incoming")

		protocol := c.Params("protocol")
		domain := c.Params("domain")
		url := fmt.Sprintf("%v://%v", protocol, domain)
		from := c.Params("from")
		to := c.Params("to")
		var argsPresent bool
		if url == "" || from == "" || to == "" {
			argsPresent = false
		} else {
			argsPresent = true
		}

		from = strings.ReplaceAll(from, "_", " ")
		to = strings.ReplaceAll(to, "_", " ")

		if argsPresent {
			correctArgs := true

			layout := time.ANSIC
			fromVal, err := time.Parse(layout, from)
			if err != nil {
				log.Printf("Incorrect \"from\" argument: %v. Error: %v ", from, err)
				correctArgs = false
			}
			toVal, err := time.Parse(layout, to)
			if err != nil {
				log.Printf("Incorrect \"to\" argument: %v. Error: %v ", to, err)
				correctArgs = false
			}
			log.Printf("Request: %v from %v (%v) to %v (%v)", url, fromVal.UTC(), fromVal.UTC().UnixNano(),  toVal.UTC(), toVal.UTC().UnixNano())

			if correctArgs {
				pages, err := queryUrlDataInRange(&dbContext, url, fromVal, toVal)
				if err == nil {
					buf, err := zipifyPages(pages)
					if err == nil {
						c.Send(buf)
						log.Printf("Sent %v bytes to the user", len(buf))
					} else {
						log.Printf("Zipping failed: %v", err)
						c.SendStatus(500)	
					}		
				} else {
					log.Printf("Data query failed: %v", err)
					return c.SendStatus(500)
				}
			} else {
				log.Printf("User provided incorrect arguments")
				return c.SendStatus(400)
			}
		} else {
			log.Printf("User didn't provide required arguments")
			return c.SendStatus(400)
		}

		if err != nil {
			panic(err)
		}
		return nil
	})

	app.Get("/kill", func(c *fiber.Ctx) error {
		log.Println("Kill requested")
		os.Exit(0)
		return nil
	})

	app.Listen(":3000")
}
