package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// Ssilkani o'zgartirishni unutmang!
const (
	DB_URL      = "postgresql://postgres.acpdhihybcxnxksnsdpi:mydbpass123@aws-1-eu-central-1.pooler.supabase.com:5432/postgres?sslmode=require"
	TEST_COUNT  = 500 // Umumiy so'rovlar soni
	CONCURRENCY = 15  // Bir vaqtda nechta "ishchi" ishlaydi (Supabase free limitiga mosladik)
)

func main() {
	// 1. Bazaga ulanish
	fmt.Println("📡 Bazaga ulanmoqda...")
	db, err := sql.Open("postgres", DB_URL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Pool sozlamalari
	db.SetMaxOpenConns(CONCURRENCY) // Maksimal ochiq quvurlar
	db.SetMaxIdleConns(CONCURRENCY)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatal("❌ Ulanib bo'lmadi:", err)
	}
	fmt.Println("✅ Muvaffaqiyatli ulandi! Parallel test boshlanmoqda...")

	// --- PARALLEL WRITE TEST ---
	fmt.Printf("\n--- 🚀 PARALLEL YOZISH (%d ta so'rov, %d ta oqim) ---\n", TEST_COUNT, CONCURRENCY)
	start := time.Now()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, CONCURRENCY) // Limitlovchi kanal

	for i := 0; i < TEST_COUNT; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{} // Ruxsat olish (limit)
			
			id := int64(100000 + idx)
			_, err := db.Exec(`
				INSERT INTO users (id, username, balance) 
				VALUES ($1, $2, $3)
				ON CONFLICT (id) DO UPDATE 
				SET balance = users.balance + 1, updated_at = NOW()`, 
				id, fmt.Sprintf("user_%d", id), rand.Float64()*100)
			
			if err != nil {
				log.Printf("Xato: %v", err)
			}
			
			<-semaphore // Ruxsatni bo'shatish
		}(i)
	}
	wg.Wait() // Hamma ishchilar tugashini kutish

	duration := time.Since(start)
	fmt.Printf("⏰ Vaqt ketdi: %v\n", duration)
	fmt.Printf("⚡ Tezlik: %.2f so'rov/sekundiga (RPS)\n", float64(TEST_COUNT)/duration.Seconds())

	// --- PARALLEL READ TEST ---
	fmt.Printf("\n--- 🚀 PARALLEL O'QISH (%d ta so'rov) ---\n", TEST_COUNT)
	start = time.Now()

	for i := 0; i < TEST_COUNT; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{} 

			id := int64(100000 + idx)
			var bal float64
			err := db.QueryRow("SELECT balance FROM users WHERE id = $1", id).Scan(&bal)
			if err != nil && err != sql.ErrNoRows {
				log.Printf("O'qishda xato: %v", err)
			}
			
			<-semaphore
		}(i)
	}
	wg.Wait()

	duration = time.Since(start)
	fmt.Printf("⏰ Vaqt ketdi: %v\n", duration)
	fmt.Printf("⚡ Tezlik: %.2f so'rov/sekundiga (RPS)\n", float64(TEST_COUNT)/duration.Seconds())

	fmt.Println("\n✅ Test tugadi.")
}