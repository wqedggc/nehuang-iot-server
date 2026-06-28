// Package main provides a CLI utility for generating bcrypt password hashes.
//
// Usage:
//
//	hash_password <plain_password>
//
// The hash is printed to stdout and can be used in SQL INSERT statements
// for the users.password_hash column.
package main

import (
	"fmt"
	"os"

	"nehuang-iot-server/internal/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: hash_password <plain_password>\n")
		os.Exit(1)
	}

	hash, err := auth.HashPassword(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
