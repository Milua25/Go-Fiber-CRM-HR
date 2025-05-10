package main

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"os"
	"time"
)

// constants
const (

	// environment variables
	mongoDBConnectionStringEnvVarName = "MONGODB_CONNECTION_STRING"
	mongoDBDatabaseEnvVarName         = "fiber-hrms"
	mongoDBCollectionEnvVarName       = "employees"
)

type MongoInstance struct {
	Client *mongo.Client
	Db     *mongo.Database
}

type Employee struct {
	ID     string  `json:"id,omitempty" bson:"_id,omitempty"`
	Name   string  `json:"name"`
	Salary float64 `json:"salary"`
	Age    float64 `json:"age"`
}

var mg MongoInstance

// Connect to the mongo db instance
func Connect() error {
	mongoURI := os.Getenv(mongoDBConnectionStringEnvVarName)
	if mongoURI == "" {
		log.Fatal("missing environment variable: ", mongoDBConnectionStringEnvVarName)
	}
	mongoCredentials := options.Client().ApplyURI(mongoURI).SetDirect(true)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	defer cancel()

	client, err := mongo.Connect(ctx, mongoCredentials)
	if err != nil {
		return err
	}

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return err
	}

	db := client.Database(mongoDBDatabaseEnvVarName)

	mg = MongoInstance{
		Client: client,
		Db:     db,
	}
	return nil
}

// getAllEmployees Function
func getAllEmployees(c *fiber.Ctx) error {
	query := bson.D{{}}

	cursor, err := mg.Db.Collection(mongoDBCollectionEnvVarName).Find(c.Context(), query)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	var employees []Employee = make([]Employee, 0)

	if err := cursor.All(c.Context(), &employees); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(employees)
}

// getEmployee Function
func getEmployee(c *fiber.Ctx) error {
	collection := mg.Db.Collection(mongoDBCollectionEnvVarName)

	employee := new(Employee)
	if err := c.BodyParser(employee); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	employee.ID = ""
	insertionResult, err := collection.InsertOne(c.Context(), employee)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	filter := bson.D{
		{
			Key: "_id", Value: insertionResult.InsertedID,
		},
	}
	createRecord := collection.FindOne(c.Context(), filter)

	createdEmployee := &Employee{}
	createRecord.Decode(createdEmployee)

	return c.Status(201).JSON(createdEmployee)
}

// Update an Employee record
func updateEmployee(c *fiber.Ctx) error {
	idParams := c.Params("id")
	employeeID, err := primitive.ObjectIDFromHex(idParams)
	if err != nil {
		return c.SendStatus(400)
	}
	employee := new(Employee)

	if err := c.BodyParser(employee); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	query := bson.D{{Key: "_id", Value: employeeID}}
	update := bson.D{
		{
			Key: "$set",
			Value: bson.D{
				{
					Key: "name", Value: employee.Name,
				},
				{
					Key: "age", Value: employee.Age,
				},
				{
					Key: "salary", Value: employee.Salary,
				},
			},
		},
	}

	err = mg.Db.Collection(mongoDBCollectionEnvVarName).FindOneAndUpdate(c.Context(), query, update).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return c.Status(400).SendString(err.Error())
		}
		return c.SendStatus(500)
	}
	employee.ID = idParams
	return c.Status(200).JSON(employee)
}

func deleteEmployee(c *fiber.Ctx) error {
	employeeID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.SendStatus(400)
	}
	query := bson.D{
		{
			Key: "_id", Value: employeeID,
		},
	}
	result, err := mg.Db.Collection(mongoDBCollectionEnvVarName).DeleteOne(c.Context(), &query)
	if err != nil {
		return c.SendStatus(500) // internal server error
	}
	if result.DeletedCount < 1 {
		return c.SendStatus(404) // not found
	}

	return c.Status(200).JSON("record deleted")
}

func main() {
	if err := Connect(); err != nil {
		log.Fatalf("unable to connect to the database %v", err)
	}
	app := fiber.New()

	app.Get("/employee", getAllEmployees)
	app.Post("/employee", getEmployee)
	app.Put("/employee/:id", updateEmployee)
	app.Delete("/employee/:id", deleteEmployee)

	log.Fatal(app.Listen(":3000"))
}
