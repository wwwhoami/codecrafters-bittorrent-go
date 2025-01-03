package peer

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"time"

	metainfo "github.com/codecrafters-io/bittorrent-starter-go/pkg/metainfo"
)

const (
	BlockSize      = 16384 // 16KB
	PipelineDepth  = 5
	MessageTimeout = 1 * time.Second
)

// PeerConn manages the connection to a peer
type PeerConn struct {
	conn        net.Conn
	extensionID *uint8
	id          string
	Peer        Peer
}

// NewPeerConn creates a new connection to the peer and performs the handshake
// with the peer.
func NewPeerConn(peer Peer, infoHash string) (*PeerConn, error) {
	conn, err := net.Dial("tcp", peer.String())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}

	pc := &PeerConn{
		conn: conn,
		Peer: peer,
	}

	pc.id, err = pc.handshake(infoHash, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to handshake with peer: %w", err)
	}

	return pc, nil
}

// NewPeerConnWithExtension creates a new connection to the peer and performs
// the extension handshake with the peer. The extension handshake is used to
// indicate that the client supports the bittorrent extension protocol.
func NewPeerConnWithExtension(peer Peer, infoHash string) (*PeerConn, error) {
	conn, err := net.Dial("tcp", peer.String())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}

	pc := &PeerConn{
		conn: conn,
		Peer: peer,
	}

	// Set 20th bit from right to 1 to indicate that we support the extension protocol
	pc.id, err = pc.handshake(infoHash, &[8]byte{0, 0, 0, 0, 0, 0x10, 0, 0})
	if err != nil {
		return nil, fmt.Errorf("failed to handshake with peer: %w", err)
	}

	return pc, nil
}

// RequestMetadata requests the metadata from the peer connection
// using the bittorrent extension protocol and returns the metadata piece.
func (pc *PeerConn) RequestMetadata() (metadataPiece *ExtensionPayload, err error) {
	peerExtensionID, ok := pc.ExtensionID()
	if !ok {
		err = fmt.Errorf("peer doesn't support extension protocol")
		return
	}

	// Send metadata request
	extReq := NewExtensionPayload(ExtMsgID(peerExtensionID), map[string]any{
		"msg_type": int(ExtMsgRequest),
		"piece":    0,
	})
	extReqData, err := extReq.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("failed to marshal extension request: %v", err)
		return
	}

	msg := NewPeerMsg(MsgExtensionHandshake, extReqData)
	if err = pc.sendPeerMsg(msg); err != nil {
		err = fmt.Errorf("failed to send extension request: %v", err)
		return
	}

	// Receive metadata piece
	metadataMsg, err := pc.waitForPeerMsg(MsgExtensionHandshake)
	if err != nil {
		err = fmt.Errorf("failed to receive metadata message: %v", err)
		return
	}

	metadataPiece, err = NewExtensionPayloadFromBytes(metadataMsg.payload)
	if err != nil {
		err = fmt.Errorf("failed to create metadata piece: %v", err)
		return
	}

	return
}

// PreDownload performs the setup for downloading a file from a peer connection
// including sending bitfield, interested, and unchoke messages
func (pc *PeerConn) PreDownload() error {
	// send interested message
	msg := NewPeerMsg(MsgInterested, nil)
	if err := pc.sendPeerMsg(msg); err != nil {
		return fmt.Errorf("failed to send interested message: %v", err)
	}

	// get unchoke message
	peerMsg, err := pc.waitForPeerMsg(MsgUnchoke)
	if err != nil {
		return fmt.Errorf("failed to get unchoke message: %v", err)
	}

	log.Printf("GOT UNCHOKE message: %v\n", peerMsg)

	return nil
}

// DownloadPiece downloads a complete piece using the pipeline
func (pc *PeerConn) DownloadPiece(mf *metainfo.MetaFile, pieceIdx int) ([]byte, error) {
	startTime := time.Now()

	pieceLength := mf.Info.PieceLength
	pieceCount := int(math.Ceil(float64(mf.Info.Length) / float64(pieceLength)))

	if pieceIdx >= pieceCount {
		return nil, fmt.Errorf("piece index out of bounds")
	}

	// Handle last piece
	if pieceIdx == pieceCount-1 {
		pieceLength = mf.Info.Length % mf.Info.PieceLength

		if pieceLength == 0 {
			pieceLength = mf.Info.PieceLength
		}
	}

	pieceData := make([]byte, pieceLength)
	blockCount := int(math.Ceil(float64(pieceLength) / float64(BlockSize)))

	blockReqs := make([]RequestPayload, blockCount)

	// Fill blockReqs with block requests
	for i := 0; i < blockCount; i++ {
		blockStart := i * BlockSize
		blockLength := BlockSize
		if i == blockCount-1 {
			blockLength = pieceLength % BlockSize

			if blockLength == 0 {
				blockLength = BlockSize
			}
		}

		blockReqs[i] = RequestPayload{
			index:  uint32(pieceIdx),
			begin:  uint32(blockStart),
			length: uint32(blockLength),
		}
	}

	log.Printf("Downloading Piece %d (Piece length: %d, Block count: %d, Last block size: %d)...\n", pieceIdx, pieceLength, blockCount, pieceLength-(blockCount-1)*BlockSize)

	// Download blocks
	for i := 0; i < blockCount; i += PipelineDepth {
		effectivePipeline := PipelineDepth
		if i+PipelineDepth > blockCount {
			effectivePipeline = blockCount % PipelineDepth
		}

		for j := 0; j < effectivePipeline; j++ {
			req := blockReqs[i+j]

			if err := pc.sendPeerMsg(NewPeerMsg(MsgRequest, req.MarshalBinary())); err != nil {
				return nil, fmt.Errorf("failed to send request message: %w", err)
			}
		}

		for j := 0; j < effectivePipeline; j++ {
			msg, err := pc.waitForPeerMsg(MsgPiece)
			if err != nil {
				return nil, fmt.Errorf("failed to get piece response: %w", err)
			}

			piece, err := NewPiecePayloadFromBytes(msg.payload)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal piece payload: %w", err)
			}

			log.Printf("GOT PIECE: %s\n", piece)

			copy(pieceData[piece.begin:], piece.block)
		}
	}

	log.Printf("Piece %d downloaded from %s in %.3fs\n", pieceIdx, pc.Peer, time.Since(startTime).Seconds())

	expectedHash := mf.Info.PieceHashes[pieceIdx]

	return pieceData, verifyPiece(pieceData, expectedHash)
}

// ID returns the peer connection ID
func (pc *PeerConn) ID() string {
	return pc.id
}

// ExtensionID returns the extension ID of the peer connection if it was
// established, and a boolean indicating if the extension ID was set.
func (pc *PeerConn) ExtensionID() (uint8, bool) {
	return *pc.extensionID, pc.extensionID != nil
}

// Close closes the peer connection
func (pc *PeerConn) Close() error {
	return pc.conn.Close()
}

// verifyPiece checks if the SHA1 hash of the piece matches the expected hash
// and returns an error if they do not match
func verifyPiece(got []byte, expected string) error {
	hash := sha1.Sum(got)
	hashStr := hex.EncodeToString(hash[:])

	if hashStr != expected {
		return fmt.Errorf("Hash mismatch: expected %s, got %s\n", expected, hashStr)
	}
	return nil
}

// handshake performs the handshake with the peer and returns the peer ID
// received in the handshake response message. The reservedBytes parameter
// is optional and can be used to set the reserved bytes in the handshake message.
func (pc *PeerConn) handshake(infoHash string, reservedBytes *[8]byte) (peerID string, err error) {
	handshakeMsg, err := NewHandshakeMsg(infoHash, reservedBytes)
	if err != nil {
		err = fmt.Errorf("failed to create handshake message: %v", err)
		return
	}

	if err = sendHandshake(pc.conn, handshakeMsg.Marshal()); err != nil {
		err = fmt.Errorf("failed to send handshake message: %v", err)
		return
	}

	rcvHandshake, err := receiveHandshake(pc.conn)
	if err != nil {
		err = fmt.Errorf("failed to receive handshake response: %v", err)
		return
	}

	handshakeResp, err := NewHandshakeMsgFromBytes(rcvHandshake)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal handshake response: %v", err)
		return
	}

	peerID = string(handshakeResp.PeerId)

	log.Printf("Handshake successful with peer ID: %x\n", peerID)

	reserved := handshakeResp.ReservedBytes

	// Check if the peer supports the extension protocol
	// (if the 20th bit of reserved bytes response and arg from the right is set to 1)
	if reservedBytes != nil && reservedBytes[5]&0x10 != 0 && reserved[5]&0x10 != 0 {
		extensionID, err := pc.extensionHandshake()
		pc.extensionID = &extensionID

		return peerID, err
	}

	return
}

// extensionHandshake performs the extension handshake with the peer
// and returns the extension ID of the peer if successful.
func (pc *PeerConn) extensionHandshake() (peerExtID uint8, err error) {
	if _, err = pc.waitForPeerMsg(MsgBitfield); err != nil {
		err = fmt.Errorf("failed to receive bitfield message: %v", err)
		return
	}

	extensionPayload := NewExtensionPayload(ExtMsgHandshake, map[string]any{
		"m": map[string]any{
			"ut_metadata": 1,
			"ut_pex":      2,
		},
	})

	payload, err := extensionPayload.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("failed to marshal extension payload: %v", err)
		return
	}

	msg := NewPeerMsg(MsgExtensionHandshake, payload)
	if err = pc.sendPeerMsg(msg); err != nil {
		err = fmt.Errorf("failed to send extension handshake message: %v", err)
		return
	}

	resMsg, err := pc.waitForPeerMsg(MsgExtensionHandshake)
	if err != nil {
		err = fmt.Errorf("failed to receive extension handshake response: %v", err)
		return
	}

	resPayload := &ExtensionPayload{}
	if err = resPayload.UnmarshalBinary(resMsg.payload); err != nil {
		err = fmt.Errorf("failed to unmarshal extension handshake response: %v", err)
		return
	}

	utMetadata, ok := resPayload.Payload["m"].(map[string]any)["ut_metadata"].(int)
	if !ok {
		err = fmt.Errorf("missing ut_metadata extension in handshake response")
		return
	}

	peerExtID = uint8(utMetadata)

	return
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

// sendPeerMsg sends a message to the peer
func (pc *PeerConn) sendPeerMsg(peerMsg *PeerMsg) error {
	msg := peerMsg.MarshalBinary()

	n, err := pc.conn.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if n != len(msg) {
		return fmt.Errorf("incomplete write: wrote %d of %d bytes", n, len(msg))
	}

	log.Printf("SENT: %s\n", peerMsg)

	return nil
}

// readPeerMsg reads a message from the peer
func (pc *PeerConn) readPeerMsg() (*PeerMsg, error) {
	// Read message length
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(pc.conn, lenBuf); err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	msgLen := binary.BigEndian.Uint32(lenBuf)

	// Read payload if length > 0
	var payload []byte
	if msgLen > 0 {
		payload = make([]byte, msgLen)
		if _, err := io.ReadFull(pc.conn, payload); err != nil {
			return nil, fmt.Errorf("failed to read payload: %w", err)
		}
	}

	// Parse message ID and payload
	var msgID MsgID

	if len(payload) > 0 {
		msgID = MsgID(payload[0])
		payload = payload[1:]
	}

	return &PeerMsg{
		id:      msgID,
		length:  msgLen,
		payload: payload,
	}, nil
}

// waitForPeerMsg waits for a specific message type
func (pc *PeerConn) waitForPeerMsg(expectedID MsgID) (*PeerMsg, error) {
	msgChan := make(chan *PeerMsg, 1)
	errChan := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), MessageTimeout)

	defer close(msgChan)
	defer close(errChan)
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
				msg, err := pc.readPeerMsg()
				if err != nil {
					errChan <- err
					return
				}

				if msg.id == expectedID {
					msgChan <- msg
					return
				}

				// Handle keep-alive messages
				if msg.length == 0 {
					continue
				}

				// Log other message types
				log.Printf("GOT: %v while waiting for type %d\n", msg, expectedID)
			}
		}
	}()

	select {
	case msg := <-msgChan:
		log.Printf("GOT: %v\n", msg)
		return msg, nil
	case err := <-errChan:
		if err == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout waiting for message ID %d", expectedID)
		}
		return nil, err
	}
}
