package main

import (
	"log"

	"github.com/nexus-estate/nexus-estate-engine/internal/app"
	"github.com/nexus-estate/nexus-estate-engine/internal/platform/shutdown"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	ctx, stop := shutdown.SignalContext()
	defer stop()
	return app.RunWorker(ctx)
}
