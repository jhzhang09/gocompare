package main

import (
	"fmt"
	"gocompare/internal/native"
	"os"
)

func main() {
	if err := native.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "GoCompare:", err)
		os.Exit(2)
	}
}
