package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/inceptionstack/embedrock"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	port := flag.Int("port", 8089, "Port to listen on")
	host := flag.String("host", "127.0.0.1", "Host to bind to")
	region := flag.String("region", "us-east-1", "AWS region for Bedrock")
	model := flag.String("model", "amazon.titan-embed-text-v2:0", "Default Bedrock embedding model")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("embedrock %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	emb, err := embedrock.NewBedrockEmbedder(*region, *model)
	if err != nil {
		log.Fatalf("Failed to create embedder: %v", err)
	}

	handler := embedrock.NewHandlerWithModel(emb, *model)
	addr := fmt.Sprintf("%s:%d", *host, *port)

	log.Printf("embedrock %s starting on http://%s (region=%s, model=%s)", version, addr, *region, *model)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
