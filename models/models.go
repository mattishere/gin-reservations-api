package models

import (
	"time"
)

type User struct {
	ID   string `bson:"_id" json:"id"`
	Name string `bson:"name" json:"name"`
}

type Chargepoint struct {
	ID         string      `bson:"_id" json:"id"`
	Connectors []Connector `bson:"connectors" json:"connectors"`
}

type Connector struct {
	ID    int    `bson:"_id" json:"id"`
	State string `bson:"state" json:"state"`
}

type Reservation struct {
	// Suggestion for IDs: currently, we use UnixNano() for the ID because for the demonstration, it is sufficient, but I would recommend swapping to something like Mongo ObjectIDs because they're less likely to conflict. For the demo, it's fine!
	ID                  int       `bson:"_id"`
	Chargepoint         string    `bson:"chargepoint"`
	Connector           int       `bson:"connector"`
	UserID              string    `bson:"userId" json:"userId"`
	ExpiryTime          time.Time `bson:"expiryTime"`
	HasStartedCharging  bool      `bson:"hasStartedCharging"`
	ChargingTime        time.Time `bson:"chargingTime"`
	HasFinishedCharging bool      `bson:"hasFinishedCharging"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}
