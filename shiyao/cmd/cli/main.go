package main

import (
	"fmt"
	"os"
)

const banner = `
   _____ _     _
  / ____| |   (_)
 | (___ | |__  _  _   _  __ _  ___
  \___ \| '_ \| || | | |/ _` + "`" + ` |/ _ \
  ____) | | | | || |_| | (_| | (_) |
 |_____/|_| |_|_| \__, |\__,_|\___/
                   __/ |
                  |___/

Shiyao CLI — Secure, sub-second microVM execution for AI agents
`

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "hello":
		fmt.Println("Hello from Shiyao! 🛡️⚡")

	case "version", "--version", "-v":
		fmt.Println("shiyao CLI v0.1.0")

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(banner)
	fmt.Println("Usage:")
	fmt.Println("  go run cmd/command/main.go <command>")
	fmt.Println()
	fmt.Println("Available Commands:")
	fmt.Println("  hello      Print a greeting")
	fmt.Println("  version    Print the CLI version")
	fmt.Println("  help       Show this help message")
}
