package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bencode"
)

// MetaInfo represents the metadata information of a torrent file.
type MetaInfo struct {
	Name        string
	Pieces      string
	Hash        string
	PieceHashes []string
	Length      int
	PieceLength int
}

// NewMetaInfoFromMap creates a new MetaInfo instance from a map.
func NewMetaInfoFromMap(m map[string]any) (mi *MetaInfo, err error) {
	mi = new(MetaInfo)

	if mi.Name, err = getStringFromMap(m, "name"); err != nil {
		return
	}
	if mi.Pieces, err = getStringOrBytesFromMap(m, "pieces"); err != nil {
		return
	}
	if mi.Length, err = getIntFromMap(m, "length"); err != nil {
		return
	}
	if mi.PieceLength, err = getIntFromMap(m, "piece length"); err != nil {
		return
	}

	mi.PieceHashes = mi.pieceHashes()
	mi.Hash, err = mi.Sha1Sum()
	if err != nil {
		err = fmt.Errorf("failed to calculate info hash: %v", err)
		return nil, err
	}

	return
}

func (mi *MetaInfo) Bencode() (string, error) {
	return bencode.BencodeVal(map[string]any{
		"length":       mi.Length,
		"name":         mi.Name,
		"piece length": mi.PieceLength,
		"pieces":       mi.Pieces,
	})
}

// Sha1Sum calculates the SHA1 hash of the bencoded info dictionary.
func (mi *MetaInfo) Sha1Sum() (string, error) {
	h := sha1.New()

	bencoded, err := mi.Bencode()
	if err != nil {
		return "", err
	}

	_, err = io.WriteString(h, bencoded)
	if err != nil {
		return "", err
	}

	return string(h.Sum(nil)), nil
}

// pieceHashes returns the SHA1 hashes of the pieces.
func (mi *MetaInfo) pieceHashes() (ph []string) {
	pieces := []byte(mi.Pieces)

	for i := 0; i < len(pieces); i += 20 {
		hash := pieces[i : i+20]
		ph = append(ph, fmt.Sprintf("%x", hash))
	}

	return
}

type MetaFile struct {
	Announce string
	Info     MetaInfo
}

func NewMetaFileFromMap(m map[string]any) (*MetaFile, error) {
	mf := new(MetaFile)

	if announce, ok := m["announce"].(string); ok {
		mf.Announce = announce
	} else {
		return nil, fmt.Errorf("invalid announce URL")
	}
	if info, ok := m["info"].(map[string]any); ok {
		info, err := NewMetaInfoFromMap(info)
		if err != nil {
			return nil, err
		}

		mf.Info = *info
	} else {
		return nil, fmt.Errorf("invalid info")
	}

	return mf, nil
}

// ParseMetaFile parses a .torrent file and returns a MetaFile instance,
// containing the metadata information of the torrent.
func ParseMetaFile(filename string) (*MetaFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	decoded, err := bencode.DecodeReader(file)
	if err != nil {
		return nil, err
	}

	decodedMap, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid .torrent file")
	}

	metafile, err := NewMetaFileFromMap(decodedMap)

	return metafile, err
}

// HandshakeMsg creates a handshake message for the torrent.
func HandshakeMsg(infoHash string, reservedBytes *[8]byte) ([]byte, error) {
	peerId, err := GenRandStr(20)
	if err != nil {
		return nil, fmt.Errorf("failed to generate peer ID: %v", err)
	}

	if reservedBytes == nil {
		reservedBytes = new([8]byte)
	}

	handshakeMsg := make([]byte, 0, handshakeMsgSize)
	handshakeMsg = append(handshakeMsg, 19)
	handshakeMsg = append(handshakeMsg, []byte("BitTorrent protocol")...)
	handshakeMsg = append(handshakeMsg, reservedBytes[:]...)
	handshakeMsg = append(handshakeMsg, infoHash...)
	handshakeMsg = append(handshakeMsg, peerId...)

	return handshakeMsg, nil
}

// getStringFromMap returns a string value from a map.
func getStringFromMap(m map[string]any, key string) (string, error) {
	if value, ok := m[key].(string); ok {
		return value, nil
	}
	return "", fmt.Errorf("invalid %s", key)
}

// getStringOrBytesFromMap returns a string or bytes value from a map.
func getStringOrBytesFromMap(m map[string]any, key string) (string, error) {
	if value, ok := m[key].(string); ok {
		return value, nil
	} else if value, ok := m[key].([]byte); ok {
		return string(value), nil
	}
	return "", fmt.Errorf("invalid %s", key)
}

// getIntFromMap returns an int value from a map.
func getIntFromMap(m map[string]any, key string) (int, error) {
	if value, ok := m[key].(int); ok {
		return value, nil
	}
	return 0, fmt.Errorf("invalid %s", key)
}
