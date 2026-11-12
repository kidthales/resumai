package main

import (
	"fmt"
	"os"

	"github.com/kidthales/resumai/internal/version"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("resumai version %s\n", version.Version)
		return
	}
	fmt.Printf("resumai v%s - resume vault & rendering engine\n", version.Version)
}
