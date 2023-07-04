package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"reservations/db"
	"reservations/docs"
	"reservations/endpoints"
	"reservations/models"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"go.mongodb.org/mongo-driver/mongo"

	_ "reservations/docs"
)

// @title Reservations API
// @description A reservation API project assignment.
// @version Preview 1.0.0
// @BasePath /
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file.")
	}

	client, err := db.Connect()
	if err != nil {
		panic(err)
	}
	defer client.Disconnect(context.Background())

	router := gin.Default()

	port := os.Getenv("API_HOST_PORT")
	if os.Getenv("IS_CONTAINERIZED") == "true" {
		port = os.Getenv("API_CONTAINER_PORT")
	}

	address := os.Getenv("API_ADDRESS") + ":" + port

	docs.SwaggerInfo.Host = "localhost:" + os.Getenv("API_HOST_PORT")

	// Swagger (OpenAPI) endpoints
	router.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Main API endpoints
	RunEndpoints(address, "MainDB", router, client)
}

func RunEndpoints(address string, databaseName string, router *gin.Engine, client *mongo.Client) {
	database := client.Database(databaseName)
	usersCollection := database.Collection("users")
	chargepointsCollection := database.Collection("chargepoints")
	reservationsCollection := database.Collection("reservations")

	// Goroutine for checking all of the open reservations and closing any that are outdated
	go endpoints.CheckReservations(reservationsCollection, chargepointsCollection)

	router.POST("/users/:id", func(c *gin.Context) {
		endpoints.CreateUser(c, usersCollection)
	})

	router.GET("/users/:id", func(c *gin.Context) {
		id := c.Param("id")

		user, err := endpoints.FindUserByID(id, usersCollection)
		if err != nil {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "User not found"})
			return
		}

		c.JSON(http.StatusOK, user)

	})

	router.GET("/users", func(c *gin.Context) {
		documents, err := endpoints.GetAllUsers(usersCollection)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch users"})
			return
		}

		c.JSON(http.StatusOK, documents)
	})

	router.POST("/chargepoints/:id", func(c *gin.Context) {
		endpoints.CreateChargepoint(c, chargepointsCollection)
	})

	router.GET("/chargepoints/:id", func(c *gin.Context) {
		id := c.Param("id")
		chargepoint, err := endpoints.FindChargepointByID(id, chargepointsCollection)
		if err != nil {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Chargepoint not found"})
			return
		}
		c.JSON(http.StatusOK, chargepoint)
	})

	router.GET("/chargepoints", func(c *gin.Context) {
		documents, err := endpoints.GetAllChargepoints(chargepointsCollection)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch chargepoints"})
			return
		}

		c.JSON(http.StatusOK, documents)
	})

	router.POST("/charge/:cpID/:coID", func(c *gin.Context) {
		endpoints.Charge(c, reservationsCollection, chargepointsCollection, usersCollection)
	})

	router.POST("/changestate/:cpID/:coID", func(c *gin.Context) {
		endpoints.ChangeConnectorState(c, chargepointsCollection)
	})

	router.POST("/reservations/:cpID/:coID", func(c *gin.Context) {
		endpoints.CreateReservation(c, reservationsCollection, chargepointsCollection, usersCollection)
	})

	router.GET("/reservations", func(c *gin.Context) {
		documents, err := endpoints.GetAllReservations(reservationsCollection)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch reservations"})
			return
		}

		c.JSON(http.StatusOK, documents)
	})

	router.Run(address)
}
