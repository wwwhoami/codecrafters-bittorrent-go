package metainfo

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/util"
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

	if mi.Name, err = util.GetStringFromMap(m, "name"); err != nil {
		return
	}
	if mi.Pieces, err = util.GetStringOrBytesFromMap(m, "pieces"); err != nil {
		return
	}
	if mi.Length, err = util.GetIntFromMap(m, "length"); err != nil {
		return
	}
	if mi.PieceLength, err = util.GetIntFromMap(m, "piece length"); err != nil {
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
