package main

import (
	"context"
	"log"
	"net/http"

	"native-ios-passkit-wip/internal/app"
	"native-ios-passkit-wip/internal/config"
)

func main() {
	cfg := config.Load()

	handler, cleanup, err := app.NewHTTPHandler(context.Background(), cfg)
	if err != nil {
		log.Fatalf("initialize application: %v", err)
	}
	defer cleanup()

	addr := ":" + cfg.Port
	log.Printf("native iOS PassKit backend listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
