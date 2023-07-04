package endpoints

import (
	"context"
	"net/http"
	"reservations/db"
	"reservations/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CreateUser godoc
// @Summary Create a new user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body CreateUserRequest true "Request body"
// @Success 200 {object} models.MessageResponse
// @Failure 500 {object} models.ErrorResponse
// @Failure 400 {object} models.ErrorResponse
// @Router /users/{id} [post]
func CreateUser(c *gin.Context, collection *mongo.Collection) {
	var newUser models.User

	id := c.Param("id")

	var req CreateUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Name must be a non-empty string"})
		return
	}

	if id == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "ID must be a non-empty string"})
		return
	}

	newUser.ID = id
	newUser.Name = req.Name

	_, err := collection.InsertOne(context.Background(), newUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create a new user, perhaps an existing ID was entered"})
		return
	}

	c.JSON(http.StatusOK, models.MessageResponse{Message: "User created"})
}

type CreateUserRequest struct {
	Name string `json:"name"`
}

// FindUserByID godoc
// @Summary Get information about a user by their ID
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.User
// @Failure 404 {object} models.ErrorResponse
// @Router /users/{id} [get]
func FindUserByID(id string, collection *mongo.Collection) (models.User, error) {
	var user models.User

	err := collection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return models.User{}, err
	}

	return user, nil
}

// GetAllUsers godoc
// @Summary Get all users
// @Tags Users
// @Produce json
// @Success 200 {object} []models.User
// @Failure 500 {object} models.ErrorResponse
// @Router /users [get]
func GetAllUsers(collection *mongo.Collection) ([]bson.M, error) {
	documents, err := db.GetAllDocumentsInCollection(collection)
	if err != nil {
		return []bson.M{}, err
	}

	return documents, nil
}
