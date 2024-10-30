package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mybittorrent <command> [args]")
		os.Exit(1)
	}

	command := os.Args[1]

	if err := processCommand(command); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func processCommand(command string) error {
	switch command {
	case "decode":
		return decodeCommand()
	case "info":
		return infoCommand()
	case "peers":
		return peersCommand()
	default:
		return fmt.Errorf("unknown command: %v", command)
	}
}

func decodeCommand() error {
	bencodedValue := os.Args[2]

	decoded, err := decodeBencode(bencodedValue)
	if err != nil {
		return err
	}

	jsonOutput, _ := json.Marshal(decoded)
	fmt.Println(string(jsonOutput))

	return nil
}

func infoCommand() error {
	filename := os.Args[2]

	mf, err := parseMetaFile(filename)
	if err != nil {
		return err
	}

	infoHash, err := mf.Info.Sha1Sum()
	if err != nil {
		return err
	}

	pieceHashes := mf.Info.PieceHashes()

	fmt.Printf("Tracker URL: %v\n", mf.Announce)
	fmt.Printf("Length: %v\n", mf.Info.Length)
	fmt.Printf("Info Hash: %x\n", infoHash)
	fmt.Printf("Piece Length: %v\n", mf.Info.PieceLength)
	fmt.Printf("Piece Hashes:\n%v\n", strings.Join(pieceHashes, "\n"))

	return nil
}

func peersCommand() error {
	filename := os.Args[2]

	peersInfo, err := discoverPeers(filename)
	if err != nil {
		return err
	}

	var peersInfoStr string
	for _, peer := range peersInfo {
		peersInfoStr = fmt.Sprintf("%s%s\n", peersInfoStr, peer)
	}

	fmt.Printf("Peers Discovered:\n%v", peersInfoStr)

	return nil
}
