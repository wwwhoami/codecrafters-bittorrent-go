package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mybittorrent <command> [args]")
		os.Exit(1)
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

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
	case "download_piece":
		return downloadPieceCommand()
	case "download":
		return downloadCommand()
	case "magnet_parse":
		return magnetParseCommand()
	case "magnet_handshake":
		return magnetHandshakeCommand()
	case "magnet_info":
		return magnetInfoCommand()
	default:
		return fmt.Errorf("unknown command: %v", command)
	}
}

func magnetInfoCommand() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("not enough arguments: expected 'mybittorrent magnet_info <magnet_link>'")
	}

	magnetLink := os.Args[2]

	infoHash, _, trackerURL, err := parseMagnetLink(magnetLink)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %v", err)
	}

	peersInfo, err := DiscoverPeers(trackerURL, infoHash, 1)
	if err != nil {
		return fmt.Errorf("failed to discover peers: %v", err)
	}
	peer := peersInfo[0]

	pc, err := NewPeerConnWithExtension(peer, infoHash)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %v", err)
	}

	metadataPiece, err := pc.RequestMetadata()
	if err != nil {
		return fmt.Errorf("failed to request metadata: %v", err)
	}

	mf, err := NewMetaFileFromMap(map[string]any{
		"announce": trackerURL,
		"info":     metadataPiece.payload["meta_piece"],
	})

	fmt.Printf("Tracker URL: %v\n", mf.Announce)
	fmt.Printf("Length: %v\n", mf.Info.Length)
	fmt.Printf("Info Hash: %x\n", mf.Info.Hash)
	fmt.Printf("Piece Length: %v\n", mf.Info.PieceLength)
	fmt.Printf("Piece Hashes:\n%v\n", strings.Join(mf.Info.PieceHashes, "\n"))

	return err
}

func magnetHandshakeCommand() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("not enough arguments: expected 'mybittorrent magnet_handshake <magnet_link>'")
	}

	magnetLink := os.Args[2]

	infoHash, _, trackerURL, err := parseMagnetLink(magnetLink)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %v", err)
	}

	peersInfo, err := DiscoverPeers(trackerURL, infoHash, 1)
	if err != nil {
		return err
	}
	peer := peersInfo[0]

	pc, err := NewPeerConnWithExtension(peer, infoHash)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %v", err)
	}

	peerExtensionID, ok := pc.ExtensionID()
	if !ok {
		return fmt.Errorf("peer doesn't support extension protocol")
	}

	fmt.Printf("Peer ID: %x\n", pc.id)
	fmt.Printf("Peer Metadata Extension ID: %v\n", peerExtensionID)

	return err
}

func magnetParseCommand() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("not enough arguments: expected 'mybittorrent magnet_parse <magnet_link>'")
	}

	matnetLink := os.Args[2]

	infoHash, filename, trackerURL, err := parseMagnetLink(matnetLink)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %v", err)
	}

	fmt.Printf("Tracker URL: %v\n", trackerURL)
	fmt.Printf("Info Hash: %x\n", infoHash)
	fmt.Printf("Filename: %v\n", filename)

	return nil
}

func downloadCommand() error {
	outFilename, filename, err := parseDownloadArgs()
	if err != nil {
		return fmt.Errorf("failed to parse download piece args: %v", err)
	}

	fmt.Printf("Downloading file: %v\n", filename)
	fmt.Printf("Output file: %v\n", outFilename)

	mf, err := ParseMetaFile(filename)
	if err != nil {
		return fmt.Errorf("failed to parse metafile: %v", err)
	}

	torrent, err := NewTorrent(mf)
	if err != nil {
		return fmt.Errorf("failed to create torrent: %v", err)
	}
	defer torrent.Close()

	if err := torrent.DownloadFile(outFilename); err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}

	fmt.Printf("File downloaded to: %v\n", outFilename)

	return nil
}

func parseDownloadArgs() (outFile, filename string, err error) {
	if len(os.Args) < 5 {
		err = fmt.Errorf("not enough arguments")
		return
	}

	fmt.Printf("Args: %v\n", os.Args)

	outFile, filename = os.Args[3], os.Args[4]
	if filename == "" {
		err = fmt.Errorf("not enough arguments")
		return
	}

	pieceOutPath := outFile[:strings.LastIndex(outFile, "/")]

	// Create piece output file directory if it doesn't exist
	if _, err = os.Stat(outFile); err != nil {
		fmt.Printf("Piece output directory doesn't exist, creating dir: %v\n", pieceOutPath)

		if err = os.MkdirAll(pieceOutPath, os.ModePerm); err != nil {
			err = fmt.Errorf("failed to create piece output directory: %v", err)
			return
		}
	}

	return
}

func downloadPieceCommand() error {
	outFile, filename, pieceIdx, err := parseDownloadPieceArgs()
	if err != nil {
		return fmt.Errorf("failed to parse download piece args: %v", err)
	}

	mf, err := ParseMetaFile(filename)
	if err != nil {
		return fmt.Errorf("failed to parse metafile: %v", err)
	}

	peersInfo, err := DiscoverPeers(mf.Announce, mf.Info.Hash, mf.Info.Length)
	if err != nil {
		return err
	}

	peer := peersInfo[0]

	pc, err := NewPeerConn(peer, mf.Info.Hash)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %v", err)
	}

	defer pc.Close()

	if err := pc.PreDownload(); err != nil {
		return fmt.Errorf("failed to prepare download: %v", err)
	}

	// download piece
	data, err := pc.DownloadPiece(mf, pieceIdx)
	if err != nil {
		return fmt.Errorf("failed to download piece: %v", err)
	}

	// write piece data to file
	if err := writeToOut(outFile, data); err != nil {
		fmt.Printf("failed to write piece data to file: %v\n", err)
	}

	fmt.Printf("Piece downloaded to: %v\n", outFile)

	return nil
}

// writeToOut writes the data to the output file,
// truncating the file if it already exists.
func writeToOut(outFile string, data []byte) error {
	file, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create piece output file: %v", err)
	}
	defer file.Close()

	if _, err = file.Write(data); err != nil {
		return fmt.Errorf("failed to write data to file: %v", err)
	}

	return nil
}

func parseDownloadPieceArgs() (pieceOutFile string, filename string, pieceIdx int, err error) {
	if len(os.Args) < 6 {
		err = fmt.Errorf("not enough arguments")
		return
	}

	pieceOutFile, filename, pieceIdxStr := os.Args[3], os.Args[4], os.Args[5]
	if filename == "" || pieceIdxStr == "" {
		err = fmt.Errorf("not enough arguments")
		return
	}

	pieceIdx, err = strconv.Atoi(pieceIdxStr)
	if err != nil {
		err = fmt.Errorf("failed to parse piece index: %v", err)
		return
	}

	pieceOutPath := pieceOutFile[:strings.LastIndex(pieceOutFile, "/")]

	// Create piece output file directory if it doesn't exist
	if _, err = os.Stat(pieceOutFile); err != nil {
		fmt.Printf("Piece output directory doesn't exist, creating dir: %v\n", pieceOutPath)

		if err = os.MkdirAll(pieceOutPath, os.ModePerm); err != nil {
			err = fmt.Errorf("failed to create piece output directory: %v", err)
			return
		}
	}

	return
}

const handshakeMsgSize = 68

func handshakeCommand() error {
	if len(os.Args) < 4 {
		return fmt.Errorf("not enough arguments")
	}

	filename, peerAddr := os.Args[2], os.Args[3]

	mf, err := ParseMetaFile(filename)
	if err != nil {
		return err
	}

	peer, err := NewPeerFromAddr(peerAddr)
	if err != nil {
		return fmt.Errorf("failed to create peer: %v", err)
	}

	pc, err := NewPeerConn(*peer, mf.Info.Hash)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %v", err)
	}
	defer pc.Close()

	fmt.Printf("Peer ID: %x\n", pc.id)

	return nil
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

	mf, err := ParseMetaFile(filename)
	if err != nil {
		return err
	}

	fmt.Printf("Tracker URL: %v\n", mf.Announce)
	fmt.Printf("Length: %v\n", mf.Info.Length)
	fmt.Printf("Info Hash: %x\n", mf.Info.Hash)
	fmt.Printf("Piece Length: %v\n", mf.Info.PieceLength)
	fmt.Printf("Piece Hashes:\n%v\n", strings.Join(mf.Info.PieceHashes, "\n"))

	return nil
}

func peersCommand() error {
	filename := os.Args[2]

	mf, err := ParseMetaFile(filename)
	if err != nil {
		return fmt.Errorf("failed to parse metafile: %v", err)
	}

	peersInfo, err := DiscoverPeers(mf.Announce, mf.Info.Hash, mf.Info.Length)
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
