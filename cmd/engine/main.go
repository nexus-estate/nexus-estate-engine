package main

import (
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/app"
	"github.com/nexus-estate/nexus-estate-platform-engine/internal/platform/shutdown"
	"log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	ctx, stop := shutdown.SignalContext()
	defer stop()
	return app.RunEngine(ctx)
}
