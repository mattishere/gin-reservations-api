# Reservations API with Gin, MongoDB, and Swagger
This project was a technical assignment for a job application.

## Understanding the assignment
My thought process behind the assignment was that this was an API for an **on-sight electric vehicle station** - a customer would come to a cashier or an interactable tablet, select the on-sight chargepoint and connector and specify how long they want to charge for. Then, the customer has a **10 minute expiry time period**, meaning that if they do not start charging in said time the reservation is marked as complete and the connector becomes available again. If they do start charging, the connector is theirs to charge on for the remainder of the charging time.

## Prerequisites
- Go 1.20
- [Swag](https://github.com/swaggo/swag) (optional)
- Docker & Docker Compose (*technically* optional)

## Installation (before running)
- Move into the directory: `cd path/to/project`
- Install all of the dependencies: `go mod download`
- Verify dependencies: `go mod verify`

## Configuration (before running)
- Before running, you should consult the `.env` file, which includes comments for some environment variables and what they do.
- Make sure none of the variables are empty unless there is a comment saying that leaving it empty is handled.

## Running the program
- To run with Docker, simply run `docker-compose up`. It will install and setup all of the containers (including the MongoDB container). You can then connect to the API endpoints at `localhost:8080/` (unless you modified the environment variables, then apply those changes).
- If you don't want to use the Docker container for the API, you can run `go run main.go`. If you're using Atlas as described in the `.env` file, then you do not need to run the MongoDB container. If you still want to use the MongoDB container but not API container, you can run the MongoDB container with `docker-compose up --scale api=0`.

## OpenAPI specification
You can access the OpenAPI specification (generated with Swagger) at `/docs/index.html`, which makes interacting with the API much easier.

All of the program's endpoints are included in the specification, including a brief explanation of their use (some have a longer description to help understand their use).

Generating the specification is not difficult: assuming you have Swag installed, it is as simple as running `swag init` in the project directory. This will rebuild the Swagger/OpenAPI specification.

## Usage

An example usage of the program (assuming you are using the Swagger UI interface mentioned above, which makes interacting with the raw API much easier):
- Create a user. This can be done through the POST endpoint `/users/{id}`. Provide a name (non-empty string) and an ID (must be unique for each user).
- Create a chargepoint. This can be done through the POST endpoint `/chargepoints/{id}`. Provide the amount of connectors you want the chargepoint to have (must be more than 0) and an ID (must be unique for each chargepoint).
- Create a reservation. This can be done through the POST endpoint `/reservations/{chargepointID}/{connectorID}`. You can create a reservation for any connector with the state "Available". In the request body, enter the time you want the reservation to last for (in minutes - must be between 30 and 180 minutes), as well as a user ID.
- Begin charging. This can be done through the POST endpoint `/charge/{chargepointID}/{connectorID}`. A user can charge on a connector if they have a valid reservation for it. If they do not start charging within 10 minutes of creating the reservation, it is marked as complete and the connector becomes available for reservation again. In the request body, enter a user ID. The user will continue charging for the remainder of their reservation's time.

The explained usage above is my assumed usage of the API, however this does not cover all of the endpoints - there are many GET endpoints (see the Swagger UI) for fetching specific database entries, as well as an experimental POST endpoint for changing connector states manually (a connector state can be either "Available", "Unavailable", "Charging" or "Reserved").

## Tests
The program includes basic unit tests for the endpoint package. To run the tests, you can use the command `go test ./endpoints` (you can also include the `-v` flag for verbose logging). Make sure to also run MongoDB (unless using Atlas (see .env file for more information)), which can be done with `docker-compose up --scale api=0`.