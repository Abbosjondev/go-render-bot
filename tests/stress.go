package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Test sozlamalari
const (
	BotURL         = "http://localhost:8080" // Sizning botingiz manzili
	MockServerPort = ":8081"                 // Soxta Telegram Server porti
	TotalRequests  = 500                     // Nechta odam hujum qilishi (10, 20, 50)
)

// Statistika uchun
var (
	successCount int
	mu           sync.Mutex
	startTimes   = make(map[int64]time.Time)
	latencies    []time.Duration
)

func main() {
	// 1. Soxta Telegram Serverini ishga tushiramiz
	go startMockTelegramServer()

	// Server ishga tushishini sal kutamiz
	time.Sleep(1 * time.Second)

	fmt.Printf("=== STRESS TEST BOSHLANDI ===\n")
	fmt.Printf("Foydalanuvchilar soni: %d ta\n", TotalRequests)
	fmt.Printf("Bot manzili: %s\n", BotURL)
	fmt.Println("------------------------------------------------")

	var wg sync.WaitGroup
	totalStart := time.Now()

	// 2. Bir vaqtning o'zida N ta so'rov yuboramiz
	for i := 1; i <= TotalRequests; i++ {
		wg.Add(1)
		userID := int64(i)
		go func(uid int64) {
			defer wg.Done()
			sendWebhookUpdate(uid)
		}(userID)
	}

	wg.Wait()
	totalDuration := time.Since(totalStart)

	// 3. Natijalarni tahlil qilamiz
	analyzeResults(totalDuration)
}

// Soxta Telegram Server (Botdan kelgan javobni ushlaydi)
func startMockTelegramServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Botdan kelgan so'rovni o'qiymiz
		var body struct {
			ChatID int64 `json:"chat_id"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		// Qachon javob kelganini belgilaymiz
		mu.Lock()
		if startTime, ok := startTimes[body.ChatID]; ok {
			latency := time.Since(startTime)
			latencies = append(latencies, latency)
			successCount++
			// fmt.Printf("Foydalanuvchi %d ga javob keldi: %v\n", body.ChatID, latency) // Batafsil ko'rish uchun oching
		}
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})

	log.Fatal(http.ListenAndServe(MockServerPort, nil))
}

// Botga "Webhook" so'rovi yuborish (Foydalanuvchi yozganini simulyatsiya qilish)
func sendWebhookUpdate(userID int64) {
	update := map[string]interface{}{
		"update_id": userID,
		"message": map[string]interface{}{
			"chat": map[string]interface{}{
				"id": userID,
			},
			"text": "/start",
		},
	}

	jsonData, _ := json.Marshal(update)

	mu.Lock()
	startTimes[userID] = time.Now() // Soatni ishga tushiramiz
	mu.Unlock()

	resp, err := http.Post(BotURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("XATO: Botga ulanib bo'lmadi (ID: %d): %v\n", userID, err)
		return
	}
	defer resp.Body.Close()
}

func analyzeResults(totalTime time.Duration) {
	// Server biroz javobni kechiktirishi mumkin, shuning uchun kutamiz
	time.Sleep(2 * time.Second) 

	fmt.Println("\n------------------------------------------------")
	fmt.Println("=== TAHLIL NATIJALARI ===")
	
	if len(latencies) == 0 {
		fmt.Println("Hech qanday javob kelmadi! Botingiz ishlamayapti yoki manzil xato.")
		return
	}

	var totalLatency time.Duration
	var minLat, maxLat time.Duration
	minLat = latencies[0]

	for _, l := range latencies {
		totalLatency += l
		if l < minLat {
			minLat = l
		}
		if l > maxLat {
			maxLat = l
		}
	}

	avgLat := totalLatency / time.Duration(len(latencies))
	rps := float64(successCount) / totalTime.Seconds()

	fmt.Printf("Muvaffaqiyatli so'rovlar: %d / %d\n", successCount, TotalRequests)
	fmt.Printf("Umumiy vaqt: %v\n", totalTime)
	fmt.Printf("O'rtacha tezlik (RPS): %.2f so'rov/sekundiga\n", rps)
	fmt.Println("---")
	fmt.Printf("Eng tez javob: %v (Super!)\n", minLat)
	fmt.Printf("Eng sekin javob: %v\n", maxLat)
	fmt.Printf("O'rtacha javob vaqti: %v\n", avgLat)
	fmt.Println("------------------------------------------------")

	if avgLat < 100*time.Millisecond {
		fmt.Println("XULOSA: Botingiz 🚀 RAKETA! Juda katta yuklamani ko'tara oladi.")
	} else if avgLat < 500*time.Millisecond {
		fmt.Println("XULOSA: Botingiz A'LO darajada. Real loyihalar uchun yetarli.")
	} else {
		fmt.Println("XULOSA: Bot biroz sekin. Optimallashtirish kerak bo'lishi mumkin.")
	}
}