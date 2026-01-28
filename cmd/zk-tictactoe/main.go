package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	zktictactoe "github.com/pflow-xyz/petri-pilot/zk-tictactoe"
)

func main() {
	port := flag.Int("port", 8090, "HTTP port for the prover service")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	svc, err := zktictactoe.NewTicTacToeService()
	if err != nil {
		slog.Error("Failed to initialize prover service", "error", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", *port)
	slog.Info("Starting zk-tictactoe prover service",
		"addr", addr,
		"circuits", svc.ListCircuits(),
	)

	if err := http.ListenAndServe(addr, svc.Handler()); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
