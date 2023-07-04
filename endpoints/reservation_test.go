package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reservations/db"
	"reservations/models"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
)

func TestReservations(t *testing.T) {
	err := godotenv.Load("../.env")
	if err != nil {
		t.Fatalf("Unable to load environment variables:\n%v", err)
	}

	client, err := db.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB:\n%v", err)
	}
	chargepointsCollection := client.Database("TestDB").Collection("chargepoints")
	usersCollection := client.Database("TestDB").Collection("users")
	reservationsCollection := client.Database("TestDB").Collection("reservations")

	router := gin.Default()

	router.POST("/reservations/:cpID/:coID", func(c *gin.Context) {
		CreateReservation(c, reservationsCollection, chargepointsCollection, usersCollection)
	})

	usersCollection.InsertOne(context.Background(), models.User{ID: "customer", Name: "Customer"})
	chargepointsCollection.InsertOne(context.Background(), models.Chargepoint{ID: "cp1", Connectors: []models.Connector{
		{
			ID:    1,
			State: "Available",
		},
		{
			ID:    2,
			State: "Reserved",
		},
		{
			ID:    3,
			State: "Unavailable",
		},
		{
			ID:    4,
			State: "Charging",
		},
	}})

	type testReservation struct {
		chargepoint string
		connector   int
		userID      string
		minutes     int
		createCode  int
	}
	tests := []testReservation{
		{chargepoint: "cp1", connector: 1, userID: "customer", minutes: 45, createCode: http.StatusOK},
		{chargepoint: "cp1", connector: 5, userID: "customer", minutes: 45, createCode: http.StatusBadRequest},
		{chargepoint: "cp2", connector: 1, userID: "customer", minutes: 45, createCode: http.StatusBadRequest},
		{chargepoint: "cp1", connector: -1, userID: "customer", minutes: 45, createCode: http.StatusBadRequest},
		{chargepoint: "cp1", connector: 2, userID: "customer", minutes: 45, createCode: http.StatusBadRequest},
		{chargepoint: "cp1", connector: 3, userID: "customer", minutes: 45, createCode: http.StatusBadRequest},
		{chargepoint: "cp1", connector: 4, userID: "customer", minutes: 45, createCode: http.StatusBadRequest},
		{chargepoint: "cp1", connector: 1, userID: "someinvalidusername", minutes: 45, createCode: http.StatusBadRequest},
		{chargepoint: "cp1", connector: 1, userID: "customer", minutes: -1, createCode: http.StatusBadRequest},
		{chargepoint: "cp1", connector: 1, userID: "customer", minutes: 5, createCode: http.StatusBadRequest},
		{chargepoint: "cp1", connector: 1, userID: "customer", minutes: 360, createCode: http.StatusBadRequest},
	}

	defer func() {
		err := db.ClearCollection(chargepointsCollection)
		if err != nil {
			t.Fatalf("Failed to clear collection:\n%v", err)
		}
		err = db.ClearCollection(usersCollection)
		if err != nil {
			t.Fatalf("Failed to clear collection:\n%v", err)
		}
		err = db.ClearCollection(reservationsCollection)
		if err != nil {
			t.Fatalf("Failed to clear collection:\n%v", err)
		}
		client.Disconnect(context.Background())
	}()

	for _, test := range tests {
		t.Run("CreateReservation", func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"userId": test.userID, "minutes": test.minutes})
			endpoint := fmt.Sprint("/reservations/", test.chargepoint, "/", test.connector)
			req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != test.createCode {
				t.Errorf("Expected code %d, but received %d", test.createCode, recorder.Code)
			} else {
				t.Logf("Received the correct code %d", test.createCode)
			}
		})
	}

	t.Run("CheckNonChargingReservations", func(t *testing.T) {
		_, err := chargepointsCollection.InsertOne(context.Background(), models.Chargepoint{ID: "chargingTestChargepoint", Connectors: []models.Connector{
			{
				ID:    1,
				State: "Reserved",
			},
		}})
		if err != nil {
			t.Fatalf("Could not insert chargepoint:\n%v", err)
		}

		_, err = reservationsCollection.InsertOne(context.Background(), models.Reservation{
			ID:                  123,
			Chargepoint:         "chargingTestChargepoint",
			Connector:           1,
			UserID:              "customer",
			ExpiryTime:          time.Now().Add(-time.Hour),
			HasStartedCharging:  false,
			HasFinishedCharging: false,
		})
		if err != nil {
			t.Fatalf("Could not insert reservation:\n%v", err)
		}

		checkNonChargingReservations(reservationsCollection, chargepointsCollection)

		var updatedReservation models.Reservation
		err = reservationsCollection.FindOne(context.Background(), bson.M{"_id": 123}).Decode(&updatedReservation)
		if err != nil {
			t.Fatalf("Could not find non-charging reservation:\n%v", err)
		}

		if !updatedReservation.HasFinishedCharging {
			t.Error("Expected the reservation to have a finished charging state")
		}

		var updatedChargepoint models.Chargepoint
		err = chargepointsCollection.FindOne(context.Background(), bson.M{"_id": "chargingTestChargepoint"}).Decode(&updatedChargepoint)
		if err != nil {
			t.Fatalf("Could not find the test chargepoint:\n%v", err)
		}

		if updatedChargepoint.Connectors[0].State != "Available" {
			t.Errorf("Expected the chargepoint connector state to be %s, but received %s", "Available", updatedChargepoint.Connectors[0].State)
		}
	})

	t.Run("CheckFinishedReservations", func(t *testing.T) {
		chargepointsCollection.InsertOne(context.Background(), models.Chargepoint{ID: "finishedTestChargepoint", Connectors: []models.Connector{
			{
				ID:    1,
				State: "Charging",
			},
		}})

		_, err := reservationsCollection.InsertOne(context.Background(), models.Reservation{
			ID:                  1234,
			Chargepoint:         "finishedTestChargepoint",
			Connector:           1,
			UserID:              "customer",
			ChargingTime:        time.Now().Add(-time.Hour),
			HasStartedCharging:  true,
			HasFinishedCharging: false,
		})
		if err != nil {
			t.Fatalf("Could not insert non-charging reservation:\n%v", err)
		}

		checkFinishedReservations(reservationsCollection, chargepointsCollection)

		var updatedReservation models.Reservation
		err = reservationsCollection.FindOne(context.Background(), bson.M{"_id": 1234}).Decode(&updatedReservation)
		if err != nil {
			t.Fatalf("Could not find non-charging reservation:\n%v", err)
		}

		if !updatedReservation.HasFinishedCharging {
			t.Error("Expected the reservation to have a finished charging state")
		}

		var updatedChargepoint models.Chargepoint
		err = chargepointsCollection.FindOne(context.Background(), bson.M{"_id": "finishedTestChargepoint"}).Decode(&updatedChargepoint)
		if err != nil {
			t.Fatalf("Could not find the test chargepoint:\n%v", err)
		}

		if updatedChargepoint.Connectors[0].State != "Available" {
			t.Errorf("Expected the chargepoint connector state to be %s, but received %s", "Available", updatedChargepoint.Connectors[0].State)
		}
	})
}
