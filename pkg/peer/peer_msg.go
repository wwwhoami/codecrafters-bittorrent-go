package peer

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/util"
)

type MsgID uint8

const (
	MsgChoke              MsgID = 0
	MsgUnchoke            MsgID = 1
	MsgInterested         MsgID = 2
	MsgNotInterested      MsgID = 3
	MsgHave               MsgID = 4
	MsgBitfield           MsgID = 5
	MsgRequest            MsgID = 6
	MsgPiece              MsgID = 7
	MsgCancel             MsgID = 8
	MsgExtensionHandshake MsgID = 20
)

type PeerMsg struct {
	payload []byte
	length  uint32
	id      MsgID
}

func NewPeerMsg(id MsgID, payload []byte) *PeerMsg {
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
	if p.id == MsgRequest {
		payload := RequestPayload{}
		if err := payload.UnmarshalBinary(p.payload); err == nil {
			return fmt.Sprintf("PeerMsg{length: %v, id: %v, payload: %+v}", p.length, p.id, payload)
		}
	}
	return fmt.Sprintf("PeerMsg{length: %v, id: %v, payload_len: %+v}", p.length, p.id, len(p.payload))
}

func (p PeerMsg) MarshalBinary() (msg []byte) {
	msg = make([]byte, 0, p.length)

	msg = append(msg, binary.BigEndian.AppendUint32(nil, p.length)...)
	msg = append(msg, byte(p.id))

	msg = append(msg, p.payload...)

	return
}

func (p *PeerMsg) UnmarshalBinary(data []byte) error {
	if len(data) < 5 {
		return fmt.Errorf("invalid peer message length")
	}

	p.length = binary.BigEndian.Uint32(data[:4])
	p.id = MsgID(data[4])

	payloadStartIdx := 5
	// p.length - 1 to exclude the ID byte
	payloadEndIdx := payloadStartIdx + int(p.length) - 1
	p.payload = data[payloadStartIdx:payloadEndIdx]

	return nil
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

type ExtMsgID uint8

// ExtMsgHandshake is a extension handshake message ID
const ExtMsgHandshake ExtMsgID = 0

const (
	// ExtMsgRequest is a extension message ID for a request for a piece of metadata
	ExtMsgRequest ExtMsgID = 0
	// ExtMsgData is a extension message ID for sending a piece of metadata
	ExtMsgData ExtMsgID = 1
	// ExtMsgReject is a extension message ID for rejecting a request for a piece of metadata
	ExtMsgReject ExtMsgID = 2
)

type ExtensionPayload struct {
	Payload map[string]any
	// The identifier can refer to a specific extension type
	id ExtMsgID
}

func NewExtensionPayload(id ExtMsgID, payload map[string]any) *ExtensionPayload {
	return &ExtensionPayload{id: id, Payload: payload}
}

func NewExtensionPayloadFromBytes(data []byte) (*ExtensionPayload, error) {
	extensionPayload := &ExtensionPayload{}
	if err := extensionPayload.UnmarshalBinary(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extension payload: %v", err)
	}

	return extensionPayload, nil
}

func (e *ExtensionPayload) String() string {
	return fmt.Sprintf("ExtensionPayload{id: %v, payload: %+v}", e.id, e.Payload)
}

func (e *ExtensionPayload) MarshalBinary() ([]byte, error) {
	payload := make([]byte, 0, len(e.Payload)+1)

	bencodedPayload, err := bencode.BencodeVal(e.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to bencode extension payload: %v", err)
	}

	payload = append(payload, byte(e.id))
	payload = append(payload, bencodedPayload...)

	return payload, nil
}

func (e *ExtensionPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 1 {
		return fmt.Errorf("invalid extension payload length")
	}

	e.Payload = make(map[string]any)

	e.id = ExtMsgID(data[0])

	payload := data[1:]

	payloadReader := bufio.NewReader(strings.NewReader(string(payload)))

	decodedPayload, err := bencode.DecodeBufReader(payloadReader)
	if err != nil {
		return fmt.Errorf("failed to decode extension payload: %v", err)
	}

	decodedPayloadMap, ok := decodedPayload.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid extension payload")
	}

	e.Payload = decodedPayloadMap

	// If there is any buffered data, it means there is a metadata piece
	// attached to the extension message
	if payloadReader.Buffered() > 0 {
		metaPiece, err := bencode.DecodeBufReader(payloadReader)
		if err != nil {
			return fmt.Errorf("failed to decode extension payload: %v", err)
		}

		e.Payload["meta_piece"] = metaPiece
	}

	return nil
}

const handshakeMsgSize = 68

type HandshakeMsg struct {
	InfoHash      string
	PeerId        string
	ReservedBytes [8]byte
}

func NewHandshakeMsg(infoHash string, reservedBytes *[8]byte) (*HandshakeMsg, error) {
	peerId, err := util.GenRandStr(20)
	if err != nil {
		return nil, fmt.Errorf("failed to generate peer ID: %v", err)
	}

	if reservedBytes == nil {
		reservedBytes = new([8]byte)
	}

	return &HandshakeMsg{
		InfoHash:      infoHash,
		ReservedBytes: *reservedBytes,
		PeerId:        peerId,
	}, nil
}

func NewHandshakeMsgFromBytes(data []byte) (*HandshakeMsg, error) {
	handshakeMsg := &HandshakeMsg{}
	if err := handshakeMsg.Unmarshal(data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal handshake message: %v", err)
	}

	return handshakeMsg, nil
}

func (h *HandshakeMsg) Marshal() []byte {
	handshakeMsg := make([]byte, 0, handshakeMsgSize)
	handshakeMsg = append(handshakeMsg, 19)
	handshakeMsg = append(handshakeMsg, []byte("BitTorrent protocol")...)
	handshakeMsg = append(handshakeMsg, h.ReservedBytes[:]...)
	handshakeMsg = append(handshakeMsg, h.InfoHash...)
	handshakeMsg = append(handshakeMsg, h.PeerId...)

	return handshakeMsg
}

func (h *HandshakeMsg) Unmarshal(data []byte) error {
	if len(data) != handshakeMsgSize {
		return fmt.Errorf("invalid handshake message size")
	}

	h.InfoHash = string(data[28:48])

	copy(h.ReservedBytes[:], data[20:28])

	h.PeerId = string(data[48:68])

	return nil
}
