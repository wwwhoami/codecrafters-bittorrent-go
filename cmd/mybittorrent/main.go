package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"strconv"
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
	case "download_piece":
		return downloadPieceCommand()
	default:
		return fmt.Errorf("unknown command: %v", command)
	}
}

type RequestPayload struct {
	index  uint32
	begin  uint32
	length uint32
}

func NewRequestPayloadFromBytes(data []byte) (*RequestPayload, error) {
	payload := &RequestPayload{}

	if err := payload.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request payload: %v", err)
	}

	return payload, nil
}

func (p RequestPayload) String() string {
	return fmt.Sprintf("PeerMsgPayload{index: %v, begin: %v, length: %v}", p.index, p.begin, p.length)
}

func (p RequestPayload) MarshalBinary() []byte {
	payload := make([]byte, 0, 12)

	payload = append(payload, binary.BigEndian.AppendUint32(nil, p.index)...)
	payload = append(payload, binary.BigEndian.AppendUint32(nil, p.begin)...)
	payload = append(payload, binary.BigEndian.AppendUint32(nil, p.length)...)

	return payload
}

func (p *RequestPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("invalid peer message payload length")
	}

	p.index = binary.BigEndian.Uint32(data[:4])
	p.begin = binary.BigEndian.Uint32(data[4:8])
	p.length = binary.BigEndian.Uint32(data[8:12])

	return nil
}

type PiecePayload struct {
	block []byte
	index uint32
	begin uint32
}

func NewPiecePayloadFromBytes(data []byte) (*PiecePayload, error) {
	piecePayload := &PiecePayload{}

	if err := piecePayload.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal piece payload: %v", err)
	}

	return piecePayload, nil
}

func (p PiecePayload) String() string {
	return fmt.Sprintf("PiecePayload{index: %v, begin: %v, block_len: %v}", p.index, p.begin, len(p.block))
}

func (p PiecePayload) MarshalBinary() []byte {
	// 8 bytes for index and begin
	payload := make([]byte, 0, 8+len(p.block))

	payload = append(payload, binary.BigEndian.AppendUint32(nil, p.index)...)
	payload = append(payload, binary.BigEndian.AppendUint32(nil, p.begin)...)
	payload = append(payload, p.block...)

	return payload
}

func (p *PiecePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("invalid peer message payload length")
	}

	p.index = binary.BigEndian.Uint32(data[:4])
	p.begin = binary.BigEndian.Uint32(data[4:8])
	p.block = data[8:]

	return nil
}

type PeerMsg struct {
	payload []byte
	length  uint32
	id      uint8
}

func NewPeerMsg(id uint8, payload []byte) *PeerMsg {
	// Default length is 1 byte
	// 1 byte for ID
	length := uint32(1)

	// add payload length if any
	if payload != nil {
		length += uint32(len(payload))
	}

	return &PeerMsg{length: length, id: id, payload: payload}
}

func NewPeerMsgFromBytes(data []byte) (*PeerMsg, error) {
	peerMsg := &PeerMsg{}

	if err := peerMsg.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal peer message: %v", err)
	}

	return peerMsg, nil
}

func (p PeerMsg) String() string {
	if p.id == REQUEST_ID {
		payload := RequestPayload{}
		if err := payload.UnmarshalBinary(p.payload); err == nil {
			return fmt.Sprintf("PeerMsg{length: %v, id: %v, payload: %+v}", p.length, p.id, payload)
		}
	}
	return fmt.Sprintf("PeerMsg{length: %v, id: %v, payload_len: %+v}", p.length, p.id, len(p.payload))
}

func (p PeerMsg) MarshalBinary() []byte {
	msg := make([]byte, 0, p.length)

	msg = append(msg, binary.BigEndian.AppendUint32(nil, p.length)...)
	msg = append(msg, p.id)

	msg = append(msg, p.payload...)

	return msg
}

func (p *PeerMsg) UnmarshalBinary(data []byte) error {
	if len(data) < 5 {
		return fmt.Errorf("invalid peer message length")
	}

	p.length = binary.BigEndian.Uint32(data[:4])
	p.id = data[4]

	payloadStartIdx := 5
	// p.length - 1 to exclude the ID byte
	payloadEndIdx := payloadStartIdx + int(p.length) - 1
	p.payload = data[payloadStartIdx:payloadEndIdx]

	return nil
}

const (
	BITFIELD_ID   = 5
	INTERESTED_ID = 2
	UNCHOKE_ID    = 1
	REQUEST_ID    = 6
	PIECE_ID      = 7
)

const BLOCK_SIZE = 16384

func downloadPieceCommand() error {
	outFile, filename, pieceIdx, err := parseDownloadPieceArgs()
	if err != nil {
		return fmt.Errorf("failed to parse download piece args: %v", err)
	}

	mf, err := parseMetaFile(filename)
	if err != nil {
		return fmt.Errorf("failed to parse metafile: %v", err)
	}

	peersInfo, err := discoverPeers(mf)
	if err != nil {
		return err
	}

	peer := peersInfo[0]

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
		return fmt.Errorf("failed peer handshake: %v", err)
	}

	rcvHandshake, err := receiveHandshake(conn)
	if err != nil {
		return fmt.Errorf("failed handshake receive: %v", err)
	}

	_ = rcvHandshake[48:]

	// get bitfield message
	peerMsg, err := readPeerMsg(conn)
	if err != nil {
		return fmt.Errorf("failed to read bitfield message: %v", err)
	}

	fmt.Printf("Got BITFIELD message: %v\n", peerMsg)

	// send interested message
	msg := NewPeerMsg(INTERESTED_ID, nil)
	if err := sendPeerMsg(conn, msg); err != nil {
		return fmt.Errorf("failed to send interested message: %v", err)
	}

	fmt.Printf("Sent INTERESTED message: %v\n", msg)

	// get unchoke message
	peerMsg, err = waitForPeerMsg(conn, UNCHOKE_ID)
	if err != nil {
		return fmt.Errorf("failed to get unchoke message: %v", err)
	}
	fmt.Printf("Got UNCHOKE message: %v\n", peerMsg)

	// download piece
	data, err := downloadPiece(conn, mf, pieceIdx)
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

// writeToOut writes the piece data to the output file,
// truncating the file if it already exists.
func writeToOut(outFile string, data []byte) error {
	file, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("failed to create piece output file: %v", err)
	}

	if _, err = file.Write(data); err != nil {
		return fmt.Errorf("failed to write data to file: %v", err)
	}

	return nil
}

func downloadPiece(conn net.Conn, mf *MetaFile, pieceIdx int) (data []byte, err error) {
	pieceLength := mf.Info.PieceLength
	pieceCount := int(math.Ceil(float64(mf.Info.Length) / float64(pieceLength)))

	if pieceIdx >= pieceCount {
		return nil, fmt.Errorf("piece index out of bounds")
	} else if pieceIdx == pieceCount-1 {
		pieceLength = mf.Info.Length % mf.Info.PieceLength
	}

	blockCount := int(math.Ceil(float64(pieceLength) / float64(BLOCK_SIZE)))

	fmt.Printf(
		"File length: %v, Piece length: %v, Piece count: %v, Block size: %v, Block count: %v\n",
		mf.Info.Length,
		pieceLength,
		pieceCount,
		BLOCK_SIZE,
		blockCount,
	)

	for i := 0; i < blockCount; i++ {
		blockStart := i * BLOCK_SIZE
		blockLength := BLOCK_SIZE

		if i == blockCount-1 {
			blockLength = pieceLength - (blockCount-1)*BLOCK_SIZE
		}

		p, err := downloadBlock(conn, uint32(pieceIdx), uint32(blockStart), uint32(blockLength))
		if err != nil {
			return nil, fmt.Errorf("failed to download block %v: %v", i, err)
		}

		data = append(data, p.block...)
	}

	return
}

func downloadBlock(conn net.Conn, pieceIdx, blockStart, blockLength uint32) (p *PiecePayload, err error) {
	peerMsg := NewPeerMsg(REQUEST_ID, RequestPayload{
		index:  pieceIdx,
		begin:  blockStart,
		length: blockLength,
	}.MarshalBinary())

	// Send request message
	if err = sendPeerMsg(conn, peerMsg); err != nil {
		err = fmt.Errorf("failed to send request message: %v", err)
		return
	}

	fmt.Printf("Sent REQUEST message: %v\n", peerMsg)

	pieceMsg, err := waitForPeerMsg(conn, PIECE_ID)
	if err != nil {
		err = fmt.Errorf("failed to get piece message: %v", err)
		return
	}

	// Wait for piece message
	// Extract piece message payload
	fmt.Printf("Got PIECE message: %v\n", pieceMsg)

	p, err = NewPiecePayloadFromBytes(pieceMsg.payload)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal piece payload: %v", err)
		return
	}

	fmt.Printf("PIECE payload: %v\n", p)

	return
}

func readPeerMsg(conn net.Conn) (peerMsg *PeerMsg, err error) {
	// Read message index length
	rawLen := make([]byte, 4)
	if _, err = conn.Read(rawLen); err != nil {
		err = fmt.Errorf("failed to read peer message: %v", err)
		return
	}

	// Read message payload
	msgLen := binary.BigEndian.Uint32(rawLen)
	data := make([]byte, msgLen)

	if _, err = io.ReadFull(conn, data); err != nil {
		err = fmt.Errorf("failed to read peer message payload: %v", err)
		return
	}

	data = append(rawLen, data...)

	// Parse peer message
	peerMsg, err = NewPeerMsgFromBytes(data)
	if err != nil {
		err = fmt.Errorf("failed to parse peer message: %v", err)
		return
	}

	return
}

func sendPeerMsg(conn net.Conn, peerMsg *PeerMsg) (err error) {
	msg := peerMsg.MarshalBinary()

	if _, err = conn.Write(msg); err != nil {
		err = fmt.Errorf("failed to write peer message: %v", err)
	}

	return
}

func waitForPeerMsg(conn net.Conn, msgId uint8) (peerMsg *PeerMsg, err error) {
	for {
		peerMsg, err = readPeerMsg(conn)
		if err != nil {
			return
		}

		if peerMsg.id == msgId {
			break
		}
	}

	return
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

	mf, err := parseMetaFile(filename)
	if err != nil {
		return fmt.Errorf("failed to parse metafile: %v", err)
	}

	peersInfo, err := discoverPeers(mf)
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
