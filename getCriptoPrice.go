package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const (
	binanceBaseURL = "https://api.binance.com/api/v3/ticker/price?symbol="
	checkInterval  = 60 * time.Second
)

func ensureTableExists(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS asset_prices (
		id SERIAL PRIMARY KEY,
		symbol VARCHAR(20) NOT NULL,
		price NUMERIC(15, 6) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Error creating table: %v", err)
	}
	fmt.Println("Table asset_prices is ready.")
}

// Structure for the asset price in the Binance API
type BinancePrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

// Function to get the price of a specific asset on Binance
func getAssetPrice(symbol string) (float64, error) {
	client := resty.New()
	url := binanceBaseURL + symbol
	resp, err := client.R().Get(url)
	if err != nil {
		return 0, err
	}

	var binancePrice BinancePrice
	err = json.Unmarshal(resp.Body(), &binancePrice)
	if err != nil {
		return 0, err
	}

	price, err := strconv.ParseFloat(binancePrice.Price, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// Function to send a message via Telegram
// func sendTelegramMessage(token, chatID, message string) error {
// 	telegramURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
// 	client := resty.New()

// 	_, err := client.R().
// 		SetQueryParams(map[string]string{
// 			"chat_id": chatID,
// 			"text":    message,
// 		}).
// 		Get(telegramURL)
// 	return err
// }

// Save price data into the database
func savePrice(db *sql.DB, asset string, price float64) error {
	query := "INSERT INTO asset_prices (symbol, price, created_at) VALUES ($1, $2, $3)"
	_, err := db.Exec(query, asset, price, time.Now())
	return err
}

func main() {
	// Load environment variables from the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading the .env file: %v", err)
	}

	// Connect to the database
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
	defer db.Close()

	// Ensure that the table exists
	ensureTableExists(db)

	// telegramToken := os.Getenv("TELEGRAM_TOKEN")
	// telegramChatID := os.Getenv("TELEGRAM_CHAT_ID")

	// Load the asset list and price limits
	assets := strings.Split(os.Getenv("ASSETS"), ",")
	maxThresholdStrs := strings.Split(os.Getenv("MAX_THRESHOLDS"), ",")
	minThresholdStrs := strings.Split(os.Getenv("MIN_THRESHOLDS"), ",")

	// Convert the price limits to float64
	maxThresholds := make([]float64, len(maxThresholdStrs))
	for i, thresholdStr := range maxThresholdStrs {
		maxThresholds[i], err = strconv.ParseFloat(thresholdStr, 64)
		if err != nil {
			log.Fatalf("Error converting MAX_THRESHOLDS: %v", err)
		}
	}

	minThresholds := make([]float64, len(minThresholdStrs))
	for i, thresholdStr := range minThresholdStrs {
		minThresholds[i], err = strconv.ParseFloat(thresholdStr, 64)
		if err != nil {
			log.Fatalf("Error converting MIN_THRESHOLDS: %v", err)
		}
	}

	if len(assets) != len(maxThresholds) || len(assets) != len(minThresholds) {
		log.Fatal("The number of assets and price limits do not match")
	}

	for {
		for i, asset := range assets {
			price, err := getAssetPrice(asset)
			if err != nil {
				log.Printf("Error getting asset price %s: %v", asset, err)
				continue
			}

			fmt.Printf("Current price of %s: $%.2f\n", asset, price)

			// Save the price to the database
			err = savePrice(db, asset, price)
			if err != nil {
				log.Printf("Error saving price to database for %s: %v", asset, err)
			}

			// Check if the price is above the maximum limit or below the minimum limit
			if price >= maxThresholds[i] {
				// message := fmt.Sprintf("Alert: the price of asset %s has reached the maximum value of $%.2f", asset, price)
				// err = sendTelegramMessage(telegramToken, telegramChatID, message)
				// if err != nil {
				// 	log.Printf("Error sending message to Telegram for %s: %v", asset, err)
				// } else {
				// 	log.Println("Message sent:", message)
				// }
			} else if price <= minThresholds[i] {
				// message := fmt.Sprintf("Alert: the price of asset %s has reached the minimum value of $%.2f", asset, price)
				// err = sendTelegramMessage(telegramToken, telegramChatID, message)
				// if err != nil {
				// 	log.Printf("Error sending message to Telegram for %s: %v", asset, err)
				// } else {
				// 	log.Println("Message sent:", message)
				// }
			}
		}
		time.Sleep(checkInterval)
	}
}
