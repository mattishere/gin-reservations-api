package endpoints

import (
	"context"
	"net/http"
	"reservations/db"
	"reservations/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CreateChargepoint godoc
// @Summary Create a new chargepoint
// @Tags Chargepoints
// @Accept json
// @Produce json
// @Param id path string true "Chargepoint ID"
// @Param body body CreateChargepointRequest true "Request body"
// @Success 200 {object} models.MessageResponse
// @Failure 500 {object} models.ErrorResponse
// @Failure 400 {object} models.ErrorResponse
// @Router /chargepoints/{id} [post]
func CreateChargepoint(c *gin.Context, collection *mongo.Collection) {
	var newChargepoint models.Chargepoint

	id := c.Param("id")

	var req CreateChargepointRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	if len(id) > 20 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Chargepoint ID must exceed 20 characters"})
		return
	}

	if req.Connectors <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Connector quantity must exceed 0"})
		return
	}

	connectors := make([]models.Connector, req.Connectors)
	for i := 0; i < req.Connectors; i++ {
		connectors[i] = models.Connector{
			ID:    i + 1,
			State: "Available",
		}
	}

	newChargepoint.Connectors = connectors
	newChargepoint.ID = id

	_, err := collection.InsertOne(context.Background(), newChargepoint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create a new chargepoint, perhaps an existing ID was entered"})
		return
	}

	c.JSON(http.StatusOK, models.MessageResponse{Message: "Chargepoint created"})
}

type CreateChargepointRequest struct {
	Connectors int `json:"connectors"`
}

// ChangeConnectorState godoc
// @Summary Forcefully change the state of a connector
// @Description Experimental feature that allows use of all the possible connector states. This is useful when debugging, but can create possible edge cases (Suggestion: only use it on "Available" connectors since those will never have a reservation). The body parameter "state" can be either "Available", "Unavailable", "Charging" or "Reserved". A possible use case for this endpoint would be maintenance on a connector, setting it to "Unavailable".
// @Tags Experimental
// @Accept json
// @Produce json
// @Param chargepointID path string true "Chargepoint ID"
// @Param connectorID path int true "Connector ID"
// @Param body body ChangeConnectorStateRequest true "Request body"
// @Success 200 {object} models.MessageResponse
// @Failure 500 {object} models.ErrorResponse
// @Failure 400 {object} models.ErrorResponse
// @Router /changestate/{chargepointID}/{connectorID} [post]
func ChangeConnectorState(c *gin.Context, collection *mongo.Collection) {
	var req ChangeConnectorStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	chargepoint, err := FindChargepointByID(c.Param("cpID"), collection)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Chargepoint does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Unable to fetch chargepoints"})
		return
	}

	connectorNumber, err := strconv.Atoi(c.Param("coID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Connector ID must be a number"})
		return
	}

	if connectorNumber <= 0 || connectorNumber > len(chargepoint.Connectors) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Connector ID must be between 1 and the amount of the chargepoint's connectors"})
		return
	}

	switch req.State {
	case "Available", "Unavailable", "Charging", "Reserved":
		chargepoint.Connectors[connectorNumber-1].State = req.State
		filter := bson.M{"_id": chargepoint.ID}
		_, err = collection.UpdateOne(context.Background(), filter, bson.M{"$set": bson.M{"connectors": chargepoint.Connectors}})
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Could not update the state of the connector"})
			return
		}

		c.JSON(http.StatusOK, models.MessageResponse{Message: "Connector state changed"})
		return
	default:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "State must be either Available, Unavailable, Charging or Reserved"})
		return
	}
}

type ChangeConnectorStateRequest struct {
	State string `json:"state"`
}

// FindChargepointByID godoc
// @Summary Get information about a chargepoint by ID
// @Tags Chargepoints
// @Produce json
// @Param id path string true "Chargepoint ID"
// @Success 200 {object} models.Chargepoint
// @Failure 404 {object} models.ErrorResponse
// @Router /chargepoints/{id} [get]
func FindChargepointByID(ID string, collection *mongo.Collection) (models.Chargepoint, error) {
	var chargepoint models.Chargepoint

	err := collection.FindOne(context.Background(), bson.M{"_id": ID}).Decode(&chargepoint)
	if err != nil {
		return models.Chargepoint{}, err
	}

	return chargepoint, nil
}

// GetAllChargepoints godoc
// @Summary Get all chargepoints
// @Tags Chargepoints
// @Produce json
// @Success 200 {object} []models.Chargepoint
// @Failure 500 {object} models.ErrorResponse
// @Router /chargepoints [get]
func GetAllChargepoints(collection *mongo.Collection) ([]bson.M, error) {
	documents, err := db.GetAllDocumentsInCollection(collection)
	if err != nil {
		return []bson.M{}, err
	}

	return documents, nil
}

// Charge godoc
// @Summary Start charging
// @Description For a user to begin charging, they need to have an open reservation for the chargepoint and connector. They need to connect in the 10 minute "expiry" time period (time of reservation + 10 minutes), otherwise the reservation ends. If the user does connect in time, then they charge for the remainder of the "charging" time period specified in the reservation.
// @Tags Chargepoints
// @Accept json
// @Produce json
// @Param chargepointID path string true "Chargepoint ID"
// @Param connectorID path int true "Connector ID"
// @Param body body ChargeRequest true "Request body"
// @Success 200 {object} models.MessageResponse
// @Failure 500 {object} models.ErrorResponse
// @Failure 400 {object} models.ErrorResponse
// @Router /charge/{chargepointID}/{connectorID} [post]
func Charge(c *gin.Context, reservationsCollection, chargepointsCollection, usersCollection *mongo.Collection) {
	var req ChargeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	_, err := FindUserByID(req.UserID, usersCollection)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "User does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Unable to fetch users"})
		return
	}

	connectorNumber, err := strconv.Atoi(c.Param("coID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Connector ID must be a number"})
		return
	}

	chargepoint, err := FindChargepointByID(c.Param("cpID"), chargepointsCollection)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Chargepoint does not exist"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Unable to fetch chargepoints"})
		return
	}

	if connectorNumber <= 0 || connectorNumber > len(chargepoint.Connectors) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Connector ID must be between 1 and the amount of the chargepoint's connectors"})
		return
	}

	var reservation models.Reservation

	reservationsFilter := bson.M{
		"userId":             req.UserID,
		"chargepoint":        c.Param("cpID"),
		"connector":          connectorNumber,
		"expiryTime":         bson.M{"$gt": time.Now()},
		"hasStartedCharging": false,
	}
	err = reservationsCollection.FindOne(context.Background(), reservationsFilter).Decode(&reservation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "User does not have an active reservation to the connector"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Unable to fetch reservations"})
		return
	}

	chargepoint.Connectors[connectorNumber-1].State = "Charging"

	_, err = chargepointsCollection.UpdateOne(context.Background(), bson.M{"_id": chargepoint.ID}, bson.M{"$set": bson.M{"connectors": chargepoint.Connectors}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Could not update the state of the connector"})
		return
	}

	_, err = reservationsCollection.UpdateOne(context.Background(), bson.M{"_id": reservation.ID}, bson.M{"$set": bson.M{"hasStartedCharging": true}})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Could not update charging state of reservation"})
		return
	}

	c.JSON(http.StatusOK, models.MessageResponse{Message: "Started charging on the connector"})
}

type ChargeRequest struct {
	UserID string `json:"userId"`
}
