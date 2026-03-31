package main

import (
	"log"

	"github.com/musistudio/ccg/internal/server"
)

func main() {
	srv := server.New()
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
