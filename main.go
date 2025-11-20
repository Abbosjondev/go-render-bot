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

	_ "github.com/lib/pq"
)

var db *sql.DB

// --- TUZILMALAR ---
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
	// SOZLAMALAR
	token := os.Getenv("TELEGRAM_TOKEN")
	dbURL := os.Getenv("DATABASE_URL")
	port := os.Getenv("PORT")

	if token == "" || dbURL == "" {
		log.Fatal("Environment variables yetishmayapti!")
	}
	if port == "" {
		port = "8080"
	}

	// BAZAGA ULANISH
	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Connection Pool (Supabase Free uchun optimal)
	db.SetMaxOpenConns(15) // 15 ta ochiq ulanish
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// WEBHOOK
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

		// --- LOGIKA ---
		if text == "/start" {
			sendMessage(token, chatID, "Salom! Tezlikni tekshirish uchun /db, yuklama berish uchun /stress deb yozing.")
		} else if text == "/db" {
			// 1. Oddiy tezlik testi
			res := testSimpleSpeed(chatID, user)
			sendMessage(token, chatID, res)
		} else if text == "/stress" {
			// 2. Katta yuklama testi (Asinxron)
			sendMessage(token, chatID, "⏳ Stress test boshlandi... 2000 ta tranzaksiya bajarilyapti. Biroz kuting.")
			go runHeavyStressTest(token, chatID) // Orqa fonda ishga tushiramiz
		} else {
			sendMessage(token, chatID, "Buyruq tushunarsiz. /stress deb yozib ko'ring.")
		}
	})

	log.Printf("Server %s portda ishga tushdi...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// --- 1. ODDIY TEZLIK TESTI (/db) ---
func testSimpleSpeed(chatID int64, username string) string {
	start := time.Now()
	// Yozish
	_, err := db.Exec(`
		INSERT INTO users (id, username, balance) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET updated_at = NOW()`, chatID, username, 0)
	if err != nil {
		return fmt.Sprintf("Xato: %v", err)
	}

	// O'qish
	var bal float64
	db.QueryRow("SELECT balance FROM users WHERE id=$1", chatID).Scan(&bal)

	return fmt.Sprintf("🚀 BAZA TESTI NATIJASI:\n⏱ Vaqt: %v\n(Bu bir martalik so'rov tezligi)", time.Since(start))
}

// --- 2. OG'IR STRESS TEST (/stress) ---
func runHeavyStressTest(token string, chatID int64) {
	requestCount := 2000 // 2000 ta operatsiya bajaramiz
	concurrency := 15    // Bir vaqtda 15 ta "ishchi" (Supabase limiti)

	start := time.Now()
	var wg sync.WaitGroup

	// Kanal orqali limitlash (Semaphore pattern)
	sem := make(chan struct{}, concurrency)
	errorsCount := 0
	var mu sync.Mutex // Xatolarni sanash uchun qulf

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{} // Ruxsat olish

			// Og'irroq operatsiya: Transaction jadvaliga yozish
			_, err := db.Exec(`INSERT INTO transactions (user_id, amount, description) VALUES ($1, $2, $3)`,
				chatID, rand.Float64()*10000, fmt.Sprintf("Stress test txn %d", idx))

			if err != nil {
				mu.Lock()
				errorsCount++
				mu.Unlock()
			}

			<-sem // Ruxsatni bo'shatish
		}(i)
	}

	wg.Wait() // Hammasi tugashini kutamiz
	duration := time.Since(start)

	// Natijani hisoblash
	rps := float64(requestCount) / duration.Seconds()

	report := fmt.Sprintf(
		"💣 **STRESS TEST NATIJASI** 💣\n\n"+
			"📊 Jami so'rovlar: %d ta\n"+
			"⏱ Ketgan vaqt: %v\n"+
			"⚡ **Tezlik (RPS): %.2f so'rov/sek**\n"+
			"❌ Xatolar: %d ta\n\n"+
			"Xulosa: Agar RPS > 100 bo'lsa, 50k foydalanuvchi uchun yetarli.",
		requestCount, duration, rps, errorsCount,
	)

	sendMessage(token, chatID, report)
}

// --- YORDAMCHI ---
func sendMessage(token string, chatID int64, text string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	reqBody := SendMessageRequest{ChatID: chatID, Text: text}
	jsonData, _ := json.Marshal(reqBody)
	http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
}