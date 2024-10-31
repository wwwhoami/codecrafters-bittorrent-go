package main

import (
	"encoding/binary"
	"fmt"
)

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
