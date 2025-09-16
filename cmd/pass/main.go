package main

import (
	"fmt"
	"os"

	"github.com/noahjalex/epoch/internal/auth"
)

func main() {
	printHelp := func() {
		fmt.Println("This utility computes the hash of the provided text for testing.")
		fmt.Println("Usage: ./pass <text-to-hash>")
	}

	if len(os.Args) != 2 {
		printHelp()
		os.Exit(1)
	}

	text := os.Args[1]
	p, _ := auth.HashPassword(text)
	fmt.Println(p)
}
