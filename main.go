package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Receipt struct {
	ID           string `json:"id" bson:"_id"`
	Retailer     string `json:"retailer"`
	PurchaseDate string `json:"purchaseDate"`
	PurchaseTime string `json:"purchaseTime"`
	Items        []Item `json:"items"`
	Total        string `json:"total"`
}

type Item struct {
	ShortDescription string `json:"shortDescription"`
	Price            string `json:"price"`
}

type PointsResponse struct {
	Points int `json:"points"`
}

var (
	mongoClient *mongo.Client
	receiptsCol *mongo.Collection
)

func ConnectToMongoDB() (*mongo.Client, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI("mongodb://mongo:27017")

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			client.Disconnect(ctx)
		}
	}()

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return nil, err
	}

	log.Println("Connected to the MongoDB database")

	return client, nil
}

func processReceipt(w http.ResponseWriter, r *http.Request) {

	var receipt Receipt
	err := json.NewDecoder(r.Body).Decode(&receipt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate a unique ID for the receipt
	receipt.ID = uuid.New().String()

	// Assuming you have a bson.D document named 'bsonDocument'
	receiptsCol = mongoClient.Database("receipt-processor").Collection("receipts")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = receiptsCol.InsertOne(ctx, receipt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with the receipt ID
	response := map[string]string{"id": receipt.ID}
	jsonResponse, _ := json.Marshal(response)

	fmt.Println("Inserted Data")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)

}

func getPoints(w http.ResponseWriter, r *http.Request) {

	// Extract the receipt ID from the URL path
	id := strings.TrimPrefix(r.URL.Path, "/api/receipts/")
	var receipt Receipt

	// Find the receipt in MongoDB by ID
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := receiptsCol.FindOne(ctx, bson.M{"_id": id}).Decode(&receipt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Respond with the points awarded
	points := calculatePoints(&receipt)
	response := PointsResponse{Points: points}
	jsonResponse, _ := json.Marshal(response)

	fmt.Println("Read Data: ", points)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)

}

func calculatePoints(receipt *Receipt) int {
	points := 0

	// Rule 1: One point for every alphanumeric character in the retailer name.
	retailerName := receipt.Retailer
	alphanumericCount := 0
	for _, char := range retailerName {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			alphanumericCount++
		}
	}
	points += alphanumericCount

	// Rule 2: 50 points if the total is a round dollar amount with no cents.
	total, _ := strconv.ParseFloat(receipt.Total, 64)
	if total == float64(int(total)) {
		points += 50
	}

	// Rule 3: 25 points if the total is a multiple of 0.25.
	if total/0.25 == float64(int(total/0.25)) {
		points += 25
	}

	// Rule 4: 5 points for every two items on the receipt.
	points += (len(receipt.Items) / 2) * 5

	// Rule 5: If the trimmed length of the item description is a multiple of 3,
	// multiply the price by 0.2 and round up to the nearest integer. The result is the number of points earned.
	for _, item := range receipt.Items {
		trimmedLength := len(strings.TrimSpace(item.ShortDescription))
		if trimmedLength%3 == 0 {
			price, _ := strconv.ParseFloat(item.Price, 64)
			points += int(math.Ceil(price * 0.2))
		}
	}

	// Rule 6: 6 points if the day in the purchase date is odd.
	purchaseDate, _ := time.Parse("2006-01-02", receipt.PurchaseDate)
	if purchaseDate.Day()%2 != 0 {
		points += 6
	}

	// Rule 7: 10 points if the time of purchase is after 2:00pm and before 4:00pm.
	purchaseTime, _ := time.Parse("15:04", receipt.PurchaseTime)
	if purchaseTime.After(time.Date(0, 1, 1, 14, 0, 0, 0, time.UTC)) && purchaseTime.Before(time.Date(0, 1, 1, 16, 0, 0, 0, time.UTC)) {
		points += 10
	}

	return points
}

func main() {
	var err error
	mongoClient, err = ConnectToMongoDB()

	if err != nil {
		log.Fatal(err)
	}

	// Create a new router
	router := mux.NewRouter()
	router.HandleFunc("/api/receipts", processReceipt).Methods("POST")
	router.HandleFunc("/api/receipts/{id}", getPoints).Methods("GET")

	// Start the HTTP server
	fmt.Println("Server is running on :8080...")
	http.Handle("/", router)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
