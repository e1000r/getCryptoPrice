package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const (
	binanceBaseURL = "https://api.binance.com/api/v3/ticker/24hr?symbol="
	checkInterval  = 60 * time.Second
)

func ensureTableExists(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS asset_prices (
		id SERIAL PRIMARY KEY,
		symbol VARCHAR(20) NOT NULL,
		price NUMERIC(15, 6) NOT NULL,
		variation NUMERIC(15, 6) NOT NULL,
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
	Symbol    string `json:"symbol"`
	Price     string `json:"lastPrice"`
	Variation string `json:"priceChangePercent"`
}

// Function to get the price of a specific asset on Binance
func getAssetPrice(symbol string) (float64, float64, error) {
	client := resty.New()
	url := binanceBaseURL + symbol
	resp, err := client.R().Get(url)
	if err != nil {
		return 0, 0, err
	}

	var binancePrice BinancePrice
	err = json.Unmarshal(resp.Body(), &binancePrice)
	if err != nil {
		return 0, 0, err
	}

	price, err := strconv.ParseFloat(binancePrice.Price, 64)
	if err != nil {
		return 0, 0, err
	}

	variation, err := strconv.ParseFloat(binancePrice.Variation, 64)
	if err != nil {
		return 0, 0, err
	}

	return price, variation, nil
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
func savePrice(db *sql.DB, asset string, price float64, variation float64) error {
	query := "INSERT INTO asset_prices (symbol, price, variation, created_at) VALUES ($1, $2, $3, $4)"
	_, err := db.Exec(query, asset, price, variation, time.Now())
	return err
}

func apiGetPrices(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			http.Error(w, "'symbol' parameter is required", http.StatusBadRequest)
			return
		}

		var binancePrice BinancePrice
		err := db.QueryRow("SELECT symbol, price, variation FROM asset_prices WHERE symbol=$1 ORDER BY created_at DESC LIMIT 1", symbol).Scan(&binancePrice.Symbol, &binancePrice.Price, &binancePrice.Variation)
		if err == sql.ErrNoRows {
			http.Error(w, "Symbol not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "Error fetching data", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(binancePrice)
	}
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

	// Endpoint to access the database
	http.HandleFunc("/get-prices", apiGetPrices(db))

	go func() {
		fmt.Println("Server starting at port 8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			fmt.Println("Error at starting server:", err)
		}
	}()

	for {
		for i, asset := range assets {
			price, variation, err := getAssetPrice(asset)
			if err != nil {
				log.Printf("Error getting asset price %s: %v", asset, err)
				continue
			}

			fmt.Printf("Current price of %s: $%.2f (Variation: %.2f%%)\n", asset, price, variation)

			// Save the price to the database
			err = savePrice(db, asset, price, variation)
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
