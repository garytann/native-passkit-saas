package main

import (
	"log"
	"net/http"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/httpapi"
)

func main() {
	cfg := config.Load()
	server := httpapi.NewServer(cfg)

	addr := ":" + cfg.Port
	log.Printf("native iOS PassKit backend listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, server.Routes()))
}
