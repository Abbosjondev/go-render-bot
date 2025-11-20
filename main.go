package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// Telegramdan keladigan xabar tuzilishi
type Update struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

// Biz yuboradigan javob tuzilishi
type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN topilmadi!")
	}

	// Render portni environment orqali beradi
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Lokal test uchun
		log.Println("Port topilmadi, 8080 ishlatilmoqda")
	}

	// Webhookni qabul qiluvchi handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. Kelayotgan JSONni o'qiymiz
		var update Update
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			log.Printf("JSON xatosi: %v", err)
			return
		}

		// Agar xabar bo'lmasa (masalan, tahrirlash yoki boshqa narsa), qaytamiz
		if update.Message.Text == "" {
			return
		}

		log.Printf("Xabar keldi: %s", update.Message.Text)

		// 2. Javob matnini tayyorlaymiz
		responseText := "Siz yozdingiz: " + update.Message.Text
		if update.Message.Text == "/start" {
			responseText = "Salom! Men hech qanday tashqi kutubxonasiz yozilgan toza Go botman."
		}

		// 3. Javobni Telegramga yuboramiz
		sendMessage(token, update.Message.Chat.ID, responseText)
	})

	log.Printf("Server %s portda ishga tushdi...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// Javob yuborish funksiyasi (Sof HTTP so'rov)
func sendMessage(token string, chatID int64, text string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	reqBody := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}

	jsonData, _ := json.Marshal(reqBody)

	// POST so'rov yuborish
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Xabar yuborishda xato: %v", err)
		return
	}
	defer resp.Body.Close()
}