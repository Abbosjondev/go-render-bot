package main

import (
	"log"
	"os"
	"time"

	tele "gopkg.in/telebot.v3"
)

func main() {
	pref := tele.Settings{
		Token:  os.Getenv("8301987866:AAH4o2k1giwNalyFLVR3Jz1ke3tqefdh7Tk"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	// 2. Agar biz Renderda bo'lsak, Webhook ishlatamiz
	// Render bizga PORT va RENDER_EXTERNAL_URL beradi
	port := os.Getenv("PORT")
	publicURL := os.Getenv("RENDER_EXTERNAL_URL")

	if port != "" {
		log.Printf("Renderda ishga tushdi. Port: %s, URL: %s", port, publicURL)
		
		// Webhook sozlamalari
		webhook := &tele.Webhook{
			Listen:   ":" + port, // Render bergan portni tinglaymiz
			Endpoint: &tele.WebhookEndpoint{PublicURL: publicURL},
		}
		pref.Poller = webhook
	} else {
		log.Println("Local kompyuterda ishga tushdi (Long Polling)")
	}

	// 3. Botni yaratamiz
	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	// 4. Handlerlar (Buyruqlar)
	b.Handle("/start", func(c tele.Context) error {
		return c.Send("Salom! Men Go tilida yozilgan va Renderda ishlayotgan tezkor botman!")
	})
    
    b.Handle(tele.OnText, func(c tele.Context) error {
        return c.Send("Siz yozdingiz: " + c.Text())
    })

	log.Println("Bot ishga tushdi...")
	b.Start()
}