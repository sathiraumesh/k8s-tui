package main

import (
	"fmt"
	"os"

	"k8s-tui/internal/app"
)

func main() {
	if err := app.New().Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
