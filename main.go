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
	"time"

	_ "github.com/lib/pq" // PostgreSQL drayveri
)

// Global baza o'zgaruvchisi
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
	// 1. TOKENLARNI OLISH
	token := os.Getenv("TELEGRAM_TOKEN")
	dbURL := os.Getenv("DATABASE_URL") // Renderdan olamiz
	port := os.Getenv("PORT")

	if token == "" || dbURL == "" {
		log.Fatal("TELEGRAM_TOKEN yoki DATABASE_URL topilmadi!")
	}
	if port == "" {
		port = "8080"
	}

	// 2. BAZAGA ULANISH
	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Baza konfiguratsiya xatosi:", err)
	}
	defer db.Close()

	// Connection Pool (Supabase Free uchun optimallashtirilgan)
	db.SetMaxOpenConns(10) 
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatal("Bazaga ulanib bo'lmadi:", err)
	}
	log.Println("✅ Bazaga muvaffaqiyatli ulandi!")

	// 3. WEBHOOK HANDLER
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
		user := update.Message.From.Username

		log.Printf("[%s] Xabar: %s", user, text)

		responseText := ""

		// --- BUYRUQLAR ---
		if text == "/start" {
			responseText = fmt.Sprintf("Salom, %s! Men Go + Postgres botman.\nTezlikni sinash uchun /db deb yozing.", user)
		} else if text == "/db" {
			// TEZLIK TESTI (REAL VAQTDA)
			responseText = testDatabaseSpeed(chatID, user)
		} else {
			responseText = "Tushunmadim. /db deb yozib ko'ring."
		}

		sendMessage(token, chatID, responseText)
	})

	log.Printf("Server %s portda ishga tushdi...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Bazani tezligini o'lchovchi funksiya
func testDatabaseSpeed(chatID int64, username string) string {
	start := time.Now()

	// 1. Yozish (Insert)
	_, err := db.Exec(`
		INSERT INTO users (id, username, balance) 
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE 
		SET balance = users.balance + 1, updated_at = NOW()`, 
		chatID, username, rand.Float64()*1000)
	
	if err != nil {
		return fmt.Sprintf("❌ Xatolik: %v", err)
	}

	// 2. O'qish (Select)
	var bal float64
	err = db.QueryRow("SELECT balance FROM users WHERE id = $1", chatID).Scan(&bal)
	if err != nil {
		return fmt.Sprintf("❌ O'qishda xato: %v", err)
	}

	duration := time.Since(start)
	
	// Natijani chiroyli qilib qaytarish
	return fmt.Sprintf("🚀 BAZA TESTI NATIJASI:\n\n⏱ Ketgan vaqt: %v\n💰 Balansingiz: %.2f\n\n(Bu natija Render serveri ichida o'lchandi)", duration, bal)
}

func sendMessage(token string, chatID int64, text string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	reqBody := SendMessageRequest{ChatID: chatID, Text: text}
	jsonData, _ := json.Marshal(reqBody)
	http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
}