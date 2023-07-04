package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reservations/db"
	"reservations/models"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func TestChargepoints(t *testing.T) {
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

	router.POST("/chargepoints/:id", func(c *gin.Context) {
		CreateChargepoint(c, chargepointsCollection)
	})

	router.GET("/chargepoints/:id", func(c *gin.Context) {
		id := c.Param("id")
		chargepoint, err := FindChargepointByID(id, chargepointsCollection)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Chargepoint not found"})
			return
		}
		c.JSON(http.StatusOK, chargepoint)
	})

	router.GET("/chargepoints", func(c *gin.Context) {
		documents, err := db.GetAllDocumentsInCollection(chargepointsCollection)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed fetching chargepoints"})
			return
		}

		c.JSON(http.StatusOK, documents)
	})

	router.POST("/charge/:cpID/:coID", func(c *gin.Context) {
		Charge(c, reservationsCollection, chargepointsCollection, usersCollection)
	})

	tests := []struct {
		id         string
		connectors int
		createCode int
		getCode    int
		chargeCode int
	}{
		{id: "cp1", connectors: 10, createCode: http.StatusOK, getCode: http.StatusOK},
		{id: "cp2", connectors: -10, createCode: http.StatusBadRequest, getCode: http.StatusNotFound},
		{id: "thisisanextremelylongidthatshouldnotbeokay", connectors: 20, createCode: http.StatusBadRequest, getCode: http.StatusNotFound},
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
		t.Run("CreateChargepoint", func(t *testing.T) {
			body, _ := json.Marshal(map[string]int{"connectors": test.connectors})
			req, _ := http.NewRequest("POST", "/chargepoints/"+test.id, bytes.NewReader(body))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != test.createCode {
				t.Errorf("Expected code %d, but received %d", test.createCode, recorder.Code)
			} else {
				t.Logf("Received the correct code %d", test.createCode)
			}

		})

		t.Run("GetChargepoint", func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{})
			req, _ := http.NewRequest("GET", "/chargepoints/"+test.id, bytes.NewReader(body))

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != test.getCode {
				t.Errorf("Expected code %d, but got %d", test.getCode, recorder.Code)
			} else {
				t.Logf("Received the correct code %d", test.getCode)
			}
		})
	}

	t.Run("Charge", func(t *testing.T) {
		_, err := chargepointsCollection.InsertOne(context.Background(), models.Chargepoint{ID: "chargingChargepoint", Connectors: []models.Connector{
			{
				ID:    1,
				State: "Reserved",
			},
		}})
		if err != nil {
			t.Fatalf("Could not insert chargepoint:\n%v", err)
		}

		_, err = usersCollection.InsertOne(context.Background(), models.User{Name: "Charger", ID: "charger"})
		if err != nil {
			t.Fatalf("Could not insert user:\n%v", err)
		}

		_, err = reservationsCollection.InsertOne(context.Background(), models.Reservation{
			ID:                  987,
			Chargepoint:         "chargingChargepoint",
			Connector:           1,
			UserID:              "charger",
			ExpiryTime:          time.Now().Add(time.Hour),
			HasStartedCharging:  false,
			HasFinishedCharging: false,
		})
		if err != nil {
			t.Fatalf("Could not insert reservation:\n%v", err)
		}

		body, _ := json.Marshal(map[string]string{"userId": "charger"})

		req, _ := http.NewRequest("POST", "/charge/chargingChargepoint/1", bytes.NewReader(body))

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("Expected code %d, but got %d", http.StatusOK, recorder.Code)
		} else {
			t.Logf("Received the correct code %d", recorder.Code)
		}
	})
}
