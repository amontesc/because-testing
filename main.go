package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	err_godot := godotenv.Load()
	if err_godot != nil {
		log.Fatal("Error loading .env file")
	}

	new_relic_license := os.Getenv("NEW_RELIC_LICENSE_KEY")
	app, err := newrelic.NewApplication(newrelic.ConfigAppName("Because Go Analytics"), newrelic.ConfigLicense(new_relic_license))
	if err != nil {
		log.Fatal(err)
	}
	router := gin.Default()
	router.Use(nrgin.Middleware(app))
	router.Use(cors.Default())

	// route to help with AWS telling if the backend is healthy or not
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	pprof.Register(router, "overview/mem")

	TestingRoutes(router)

	router.Run("0.0.0.0:5000")

	select {} // Block forever
}
