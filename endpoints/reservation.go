package endpoints

import (
	"context"
	"fmt"
	"net/http"
	"reservations/db"
	"reservations/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CreateReservation godoc
// @Summary Create a reservation
// @Tags Reservations
// @Accept json
// @Produce json
// @Param chargepointID path string true "Chargepoint ID"
// @Param connectorID path int true "Connector ID"
// @Param body body ReservationRequest true "Request body"
// @Success 200 {object} models.MessageResponse
// @Failure 500 {object} models.ErrorResponse
// @Failure 400 {object} models.ErrorResponse
// @Router /reservations/{chargepointID}/{connectorID} [post]
func CreateReservation(c *gin.Context, reservationsCollection, chargepointsCollection, usersCollection *mongo.Collection) {
	var newReservation models.Reservation

	var req ReservationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	// See suggestion in models/models.go#Reservation
	newReservation.ID = int(time.Now().UnixNano())

	chargepointID := c.Param("cpID")
	newReservation.Chargepoint = chargepointID
	connectorID := c.Param("coID")

	chargepoint, err := FindChargepointByID(chargepointID, chargepointsCollection)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Chargepoint does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Unable to fetch chargepoints"})
		return
	}

	connectorNumber, err := strconv.Atoi(connectorID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Connector must be a number"})
		return
	}

	if connectorNumber <= 0 || connectorNumber > len(chargepoint.Connectors) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Connector ID must be between 1 and the amount of the chargepoint's connectors"})
		return
	}

	newReservation.Connector = connectorNumber

	if chargepoint.Connectors[connectorNumber-1].State != "Available" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Connector must be available"})
		return
	}

	_, err = FindUserByID(req.UserID, usersCollection)
	newReservation.UserID = req.UserID
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "User does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Could not fetch users"})
		return
	}

	if req.Minutes < 30 || req.Minutes > 180 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "The reservation time must be between 30 and 180 minutes"})
		return
	}

	newReservation.ExpiryTime = time.Now().Add(10 * time.Minute)
	newReservation.ChargingTime = time.Now().Add(time.Duration(req.Minutes) * time.Minute)

	_, err = reservationsCollection.InsertOne(context.Background(), newReservation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create a reservation"})
		return
	}

	chargepoint.Connectors[connectorNumber-1].State = "Reserved"

	filter := bson.M{"_id": chargepoint.ID}
	_, err = chargepointsCollection.UpdateOne(context.Background(), filter, bson.M{"$set": bson.M{"connectors": chargepoint.Connectors}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Could not update the state of the connector"})
		return
	}

	c.JSON(http.StatusOK, models.MessageResponse{Message: "Reservation created"})
}

type ReservationRequest struct {
	UserID  string `json:"userId"`
	Minutes int    `json:"minutes"`
}

// GetAllReservations godoc
// @Summary Get all reservations
// @Tags Reservations
// @Produce json
// @Success 200 {object} []models.Reservation
// @Failure 500 {object} models.ErrorResponse
// @Router /reservations [get]
func GetAllReservations(collection *mongo.Collection) ([]bson.M, error) {
	documents, err := db.GetAllDocumentsInCollection(collection)
	if err != nil {
		return []bson.M{}, err
	}

	return documents, nil
}

func CheckReservations(reservationsCollection, chargepointsCollection *mongo.Collection) {

	// Runs reservation checks every 1 minute
	for range time.NewTicker(1 * time.Minute).C {
		checkNonChargingReservations(reservationsCollection, chargepointsCollection)
		checkFinishedReservations(reservationsCollection, chargepointsCollection)
	}

}

func checkNonChargingReservations(reservationsCollection, chargepointsCollection *mongo.Collection) {
	// Get all of the expired reservations that never started charging (expiry time has passed, they haven't started charging)
	reservations, err := reservationsCollection.Find(context.Background(), bson.M{"expiryTime": bson.M{"$lte": time.Now()}, "hasStartedCharging": false, "hasFinishedCharging": false})
	if err != nil {
		fmt.Println("Error getting reservations: ", err)
		return
	}

	// Go through every expired reservation
	for reservations.Next(context.Background()) {
		var reservation models.Reservation
		err := reservations.Decode(&reservation)
		if err != nil {
			fmt.Println("Error decoding reservation: ", err)
			continue
		}

		// Set it to a finished reservation
		_, err = reservationsCollection.UpdateOne(context.Background(), bson.M{"_id": reservation.ID}, bson.M{"$set": bson.M{"hasFinishedCharging": true}})
		if err != nil {
			fmt.Println("Error updating reservation charging status: ", err)
			continue
		}

		chargepoint, err := FindChargepointByID(reservation.Chargepoint, chargepointsCollection)
		if err != nil {
			fmt.Println("Error getting chargepoint: ", err)
			continue
		}

		// Set the connector to "Available" again, ready for future reservations
		chargepoint.Connectors[reservation.Connector-1].State = "Available"

		_, err = chargepointsCollection.UpdateOne(context.Background(), bson.M{"_id": chargepoint.ID}, bson.M{"$set": bson.M{"connectors": chargepoint.Connectors}})
		if err != nil {
			fmt.Println("Error updating chargepoint connector state: ", err)
			continue
		}
	}

	reservations.Close(context.Background())
}

func checkFinishedReservations(reservationsCollection, chargepointsCollection *mongo.Collection) {
	// Get all of the outdated reservations that should be finished (charging time has passed, they haven't stopped charging, but they started charging)
	reservations, err := reservationsCollection.Find(context.Background(), bson.M{"chargingTime": bson.M{"$lte": time.Now()}, "hasStartedCharging": true, "hasFinishedCharging": false})
	if err != nil {
		fmt.Println("Error getting reservations: ", err)
		return
	}

	// Go through every outdated reservation
	for reservations.Next(context.Background()) {
		var reservation models.Reservation
		err := reservations.Decode(&reservation)
		if err != nil {
			fmt.Println("Error decoding reservation: ", err)
			continue
		}

		// Set it to a finished reservation
		_, err = reservationsCollection.UpdateOne(context.Background(), bson.M{"_id": reservation.ID}, bson.M{"$set": bson.M{"hasFinishedCharging": true}})
		if err != nil {
			fmt.Println("Error updating reservation charging status: ", err)
			continue
		}

		chargepoint, err := FindChargepointByID(reservation.Chargepoint, chargepointsCollection)
		if err != nil {
			fmt.Println("Error getting chargepoint: ", err)
			continue
		}

		// Set the connector to "Available" again, ready for future reservations
		chargepoint.Connectors[reservation.Connector-1].State = "Available"

		_, err = chargepointsCollection.UpdateOne(context.Background(), bson.M{"_id": chargepoint.ID}, bson.M{"$set": bson.M{"connectors": chargepoint.Connectors}})
		if err != nil {
			fmt.Println("Error updating chargepoint connector state: ", err)
			continue
		}
	}
	reservations.Close(context.Background())
}
