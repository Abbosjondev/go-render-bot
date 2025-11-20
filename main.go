package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL Drayveri
)

var db *sql.DB

type Update struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
		From struct {
			Username string `json:"username"`
		} `json:"from"`
	} `json:"message"`
}

type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	dbURL := os.Getenv("DATABASE_URL")
	port := os.Getenv("PORT")

	if token == "" || dbURL == "" {
		log.Fatal("Environment variables yetishmayapti!")
	}
	if port == "" {
		port = "8080"
	}

	var err error
	// Driver nomi "mysql"
	db, err = sql.Open("mysql", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// MySQL ulanish sozlamalari
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Jadvallarni yaratish
	createTables()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var update Update
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			return
		}
		if update.Message.Text == "" {
			return
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text
		user := update.Message.From.Username // <--- Mana shu o'zgaruvchi muammo edi

		// Biz uni endi Logda ishlatamiz, shunda xato yo'qoladi
		log.Printf("User: %s, Xabar: %s", user, text)

		if text == "/start" {
			// User ismini xabarga qo'shamiz
			msg := fmt.Sprintf("Salom %s! MySQL (TiDB) Testiga xush kelibsiz. /stress deb yozing.", user)
			sendMessage(token, chatID, msg)
		} else if text == "/stress" {
			sendMessage(token, chatID, "⏳ MySQL Stress test boshlandi... (TiDB Cloud)")
			go runHeavyStressTest(token, chatID)
		}
	})

	log.Printf("Server %s portda ishga tushdi...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func createTables() {
	queryUsers := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		username VARCHAR(255),
		balance DECIMAL(10, 2),
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	);`

	queryTxn := `
	CREATE TABLE IF NOT EXISTS transactions (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		user_id BIGINT NOT NULL,
		amount DECIMAL(10, 2),
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(queryUsers)
	if err != nil {
		log.Println("User table xato:", err)
	}
	_, err = db.Exec(queryTxn)
	if err != nil {
		log.Println("Txn table xato:", err)
	}
}

func runHeavyStressTest(token string, chatID int64) {
	requestCount := 2000
	concurrency := 15

	start := time.Now()
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	errorsCount := 0
	var mu sync.Mutex

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}

			// MySQL sintaksisi: ? belgisi
			_, err := db.Exec(`INSERT INTO transactions (user_id, amount, description) VALUES (?, ?, ?)`,
				chatID, rand.Float64()*10000, fmt.Sprintf("MySQL test txn %d", idx))

			if err != nil {
				mu.Lock()
				errorsCount++
				mu.Unlock()
				if errorsCount <= 5 {
					log.Println("MySQL Error:", err)
				}
			}

			<-sem
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)
	rps := float64(requestCount) / duration.Seconds()

	report := fmt.Sprintf(
		"🐬 **MySQL (TiDB) NATIJASI** 🐬\n\n"+
			"📊 So'rovlar: %d ta\n"+
			"⏱ Vaqt: %v\n"+
			"⚡ **Tezlik (RPS): %.2f**\n"+
			"❌ Xatolar: %d ta",
		requestCount, duration, rps, errorsCount,
	)
	sendMessage(token, chatID, report)
}

func sendMessage(token string, chatID int64, text string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	reqBody := SendMessageRequest{ChatID: chatID, Text: text}
	jsonData, _ := json.Marshal(reqBody)
	http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
}