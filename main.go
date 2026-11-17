package main

import (
	"fmt"
	"os"

	"k8s-tui/internal/app"
)

// version is the release the binary was built from, set by the release
// build: go build -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("k8s-tui", version)
		return
	}
	if err := app.New().Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
