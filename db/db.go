package db

import (
	"context"
	"fmt"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Connect() (*mongo.Client, error) {
	var clientOptions *options.ClientOptions
	if os.Getenv("IS_CONTAINERIZED") == "true" {
		clientOptions = options.Client().ApplyURI(os.Getenv("MONGO_CONTAINER_ADDRESS"))
	} else {
		port := os.Getenv("MONGO_HOST_PORT")
		address := os.Getenv("MONGO_HOST_ADDRESS") + ":" + port
		if port == "" {
			address = os.Getenv("MONGO_HOST_ADDRESS")
		}
		clientOptions = options.Client().ApplyURI(address)
	}

	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(context.Background(), nil)
	if err != nil {
		return nil, err
	} else {
		fmt.Println("Connected to MongoDB")
	}

	return client, nil
}

func ClearCollection(collection *mongo.Collection) error {
	_, err := collection.DeleteMany(context.Background(), bson.M{})
	if err != nil {
		return err
	}

	return nil
}

func DeleteDocument(collection *mongo.Collection, filter bson.M) error {
	_, err := collection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}
	return nil
}

func GetAllDocumentsInCollection(collection *mongo.Collection) ([]bson.M, error) {
	cursor, err := collection.Find(context.Background(), bson.D{})

	if err != nil {
		return []bson.M{}, err
	}
	defer cursor.Close(context.Background())

	var documents []bson.M
	if err := cursor.All(context.Background(), &documents); err != nil {
		return []bson.M{}, err
	}

	return documents, nil
}
