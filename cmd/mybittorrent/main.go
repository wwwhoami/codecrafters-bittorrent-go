package main

import (
	"fmt"
	"log"
	"os"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/cli"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mybittorrent <command> [args]")
		os.Exit(1)
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	command := os.Args[1]

	if err := cli.ProcessCommand(command); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
