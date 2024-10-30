package main

import (
	"encoding/json"
	"fmt"
	"net"
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
	case "handshake":
		return handshakeCommand()
	default:
		return fmt.Errorf("unknown command: %v", command)
	}
}

const handshakeMsgSize = 68

func handshakeCommand() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("not enough arguments")
	}

	filename, peerAddr := os.Args[2], os.Args[3]

	mf, err := parseMetaFile(filename)
	if err != nil {
		return err
	}

	peer, err := NewPeerFromAddr(peerAddr)
	if err != nil {
		return fmt.Errorf("failed to create peer: %v", err)
	}

	conn, err := net.Dial("tcp", peer.String())
	if err != nil {
		return fmt.Errorf("failed to connect to peer: %v", err)
	}
	defer conn.Close()

	handshakeMsg, err := mf.handshakeMsg()
	if err != nil {
		return fmt.Errorf("failed to create handshake message: %v", err)
	}

	if err := sendHandshake(conn, handshakeMsg); err != nil {
		return err
	}

	rcvHandshake, err := receiveHandshake(conn)
	if err != nil {
		return err
	}

	fmt.Printf("Peer ID: %x\n", rcvHandshake[48:])

	return nil
}

func sendHandshake(conn net.Conn, handshakeMsg []byte) error {
	_, err := conn.Write(handshakeMsg)
	if err != nil {
		return fmt.Errorf("failed to write handshake message: %v", err)
	}

	return nil
}

func receiveHandshake(conn net.Conn) ([]byte, error) {
	rcvHandshake := make([]byte, handshakeMsgSize)

	n, err := conn.Read(rcvHandshake)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake response: %v", err)
	}
	if n != handshakeMsgSize {
		return nil, fmt.Errorf("invalid handshake response length: %v", n)
	}

	return rcvHandshake, nil
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
