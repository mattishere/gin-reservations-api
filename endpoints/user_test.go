package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"reservations/db"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func TestUsers(t *testing.T) {
	err := godotenv.Load("../.env")
	if err != nil {
		t.Fatalf("Unable to load environment variables:\n%v", err)
	}

	client, err := db.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB:\n%v", err)
	}
	usersCollection := client.Database("TestDB").Collection("users")

	router := gin.Default()

	router.POST("/users/:id", func(c *gin.Context) {
		CreateUser(c, usersCollection)
	})

	router.GET("/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		user, err := FindUserByID(id, usersCollection)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusOK, user)
	})

	tests := []struct {
		id         string
		name       string
		createCode int
		getCode    int
	}{
		{id: "jaka123", name: "Jaka", createCode: http.StatusOK, getCode: http.StatusOK},
		{id: "azbe", name: "", createCode: http.StatusBadRequest, getCode: http.StatusNotFound},
	}

	defer func() {
		err := db.ClearCollection(usersCollection)
		if err != nil {
			t.Fatalf("Failed to clear collection:\n%v", err)
		}
		client.Disconnect(context.Background())
	}()

	for _, test := range tests {
		t.Run("CreateUser", func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"name": test.name})
			req, _ := http.NewRequest("POST", "/users/"+test.id, bytes.NewReader(body))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != test.createCode {
				t.Errorf("Expected code %d, but received %d", test.createCode, recorder.Code)
			} else {
				t.Logf("Received the correct code %d", test.createCode)
			}

		})

		t.Run("GetUser", func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"name": test.name})
			req, _ := http.NewRequest("GET", "/users/"+test.id, bytes.NewReader(body))

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != test.getCode {
				t.Errorf("Expected code %d, but got %d", test.getCode, recorder.Code)
			} else {
				t.Logf("Received the correct code %d", test.getCode)
			}
		})

	}
}
