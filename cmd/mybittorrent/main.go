package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// decodeStr reads a string from the reader with the length
// specified by the integer that precedes the string with a colon.
// Example: 4:spam -> spam
func decodeStr(r *bufio.Reader) (string, error) {
	bytes, err := decodeByteString(r)

	return string(bytes), err
}

func decodeByteString(r *bufio.Reader) ([]byte, error) {
	length, err := parseInt(r, ':')
	if err != nil {
		return nil, fmt.Errorf("Invalid string length format: %v", err)
	} else if length < 0 {
		return nil, fmt.Errorf("Invalid string length: %d", length)
	}

	if peekBuf, peekErr := r.Peek(length); peekErr != nil {
		_, err := r.Discard(length)
		return peekBuf, err
	}

	buf := make([]byte, length)

	_, err = r.Read(buf)

	return buf, err
}

// parseInt reads a number from the reader until the delimiter character is found.
// The delimiter character is not included in the returned string.
func parseInt(r *bufio.Reader, delim byte) (int, error) {
	str, err := r.ReadString(delim)
	if err != nil {
		return 0, err
	}
	// Discard delim character
	str = str[:len(str)-1]
	return strconv.Atoi(str)
}

// decodeInt reads a number from the reader until the 'e' delimiter character is found.
// The delimiter character is not included in the returned string.
// Example: i42e -> 42
func decodeInt(r *bufio.Reader) (int, error) {
	if ch, _, err := r.ReadRune(); ch != 'i' {
		return 0, fmt.Errorf("Integer should start with 'i' character")
	} else if err != nil {
		return 0, err
	}

	val, err := parseInt(r, 'e')
	if err != nil {
		return 0, fmt.Errorf("Invalid becoded integer: %v", err)
	}

	return val, nil
}

// decodeList reads a list from the reader until the 'e' character is found.
// The 'e' character is not included in the returned list.
// Example: l4:spam4:eggse -> ["spam", "eggs"]
func decodeList(r *bufio.Reader) (list []any, err error) {
	if ch, _, err := r.ReadRune(); ch != 'l' {
		return nil, fmt.Errorf("List should start with 'l' character")
	} else if err != nil {
		return nil, err
	}

	list = make([]any, 0)

	for {
		ch, err := r.Peek(1)
		if err != nil {
			return nil, err
		}

		if ch[0] == 'e' {
			// Discard 'e', proceeding reader by one byte
			if _, err := r.Discard(1); err != nil {
				return nil, err
			}

			break
		}

		value, err := decodeValue(r)
		if err != nil {
			return nil, err
		}

		list = append(list, value)
	}

	return
}

func decodeDict(r *bufio.Reader) (dict map[string]any, err error) {
	if ch, _, err := r.ReadRune(); ch != 'd' {
		return nil, fmt.Errorf("Dictionary should start with 'd' character")
	} else if err != nil {
		return nil, err
	}

	dict = make(map[string]any)

	for {
		ch, err := r.Peek(1)
		if err != nil {
			return nil, err
		}

		if ch[0] == 'e' {
			// Discard 'e', proceeding reader by one byte
			if _, err := r.Discard(1); err != nil {
				return nil, err
			}

			break
		}

		key, err := decodeStr(r)
		if err != nil {
			return nil, err
		}

		value, err := decodeValue(r)
		if err != nil {
			return nil, err
		}

		dict[key] = value
	}

	return
}

// decodeValue decodes a bencoded value from the reader.
func decodeValue(r *bufio.Reader) (any, error) {
	ch, err := r.Peek(1)
	if err != nil {
		return nil, err
	}
	firstCh := ch[0]

	switch {
	case unicode.IsDigit(rune(firstCh)) || firstCh == '-':
		return decodeStr(r)
	case firstCh == 'i':
		return decodeInt(r)
	case firstCh == 'l':
		return decodeList(r)
	case firstCh == 'd':
		return decodeDict(r)
	default:
		return "", fmt.Errorf("Invalid data type: %c", firstCh)
	}
}

func decodeBencode(bencode string) (any, error) {
	r := bufio.NewReader(strings.NewReader(bencode))

	return decodeValue(r)
}

// bencodeString encodes a string into a bencoded string.
// Example: spam -> 4:spam
func bencodeString(s string) string {
	return fmt.Sprintf("%d:%s", len(s), s)
}

// bencodeInt encodes an integer into a bencoded integer.
// Example: 42 -> i42e
func bencodeInt(i int) string {
	return fmt.Sprintf("i%ve", i)
}

// bencodeList encodes a list into a bencoded list.
// Example: ["spam", "eggs"] -> l4:spam
func bencodeList(l []any) (string, error) {
	var res strings.Builder

	res.WriteString("l")

	for _, v := range l {
		val, err := bencodeVal(v)
		if err != nil {
			return "", err
		}

		res.WriteString(val)

	}

	res.WriteString("e")

	return res.String(), nil
}

// bencodeDict encodes a dictionary into a bencoded dictionary.
// Example: {"spam": "eggs"} -> d4:spam4:eggse
func bencodeDict(d map[string]any) (string, error) {
	var res strings.Builder

	// Sort keys to ensure deterministic output
	// when encoding the dictionary
	keys := make([]string, 0, len(d))

	for k := range d {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	res.WriteString("d")

	for _, k := range keys {
		res.WriteString(bencodeString(k))

		v := d[k]
		val, err := bencodeVal(v)
		if err != nil {
			return "", err
		}

		res.WriteString(val)
	}

	res.WriteString("e")

	return res.String(), nil
}

// bencodeVal encodes a value into a bencoded value.
// The value can be a string, integer, list or dictionary.
func bencodeVal(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return bencodeString(v), nil
	case int:
		return bencodeInt(v), nil
	case []any:
		return bencodeList(v)
	case map[string]any:
		return bencodeDict(v)
	default:
		return "", fmt.Errorf("Invalid data type %T, for val %v", v, v)
	}
}

type MetaInfo struct {
	Name        string
	Pieces      string
	Length      int
	PieceLength int
}

func NewMetaInfoFromMap(m map[string]any) (*MetaInfo, error) {
	mi := new(MetaInfo)

	if name, ok := m["name"].(string); ok {
		mi.Name = name
	} else {
		return nil, fmt.Errorf("Invalid name")
	}
	if pieces, ok := m["pieces"].(string); ok {
		mi.Pieces = pieces
	} else if pieces, ok := m["pieces"].([]byte); ok {
		mi.Pieces = string(pieces)
	} else {
		return nil, fmt.Errorf("Invalid pieces")
	}
	if length, ok := m["length"].(int); ok {
		mi.Length = length
	} else {
		return nil, fmt.Errorf("Invalid length")
	}
	if pieceLength, ok := m["piece length"].(int); ok {
		mi.PieceLength = pieceLength
	} else {
		return nil, fmt.Errorf("Invalid piece length")
	}

	return mi, nil
}

func (mi *MetaInfo) Bencode() (string, error) {
	return bencodeDict(map[string]any{
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

func (mi *MetaInfo) PieceHashes() (ph []string) {
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
		return nil, fmt.Errorf("Invalid announce URL")
	}
	if info, ok := m["info"].(map[string]any); ok {
		info, err := NewMetaInfoFromMap(info)
		if err != nil {
			return nil, err
		}

		mf.Info = *info
	} else {
		return nil, fmt.Errorf("Invalid info")
	}

	return mf, nil
}

func parseMetaFile(filename string) (*MetaFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	r := bufio.NewReader(file)

	decoded, err := decodeDict(r)
	if err != nil {
		return nil, err
	}

	torrent, err := NewMetaFileFromMap(decoded)

	return torrent, err
}

// genRandStr generates a random string of the specified length.
// The resulting string is base64 encoded.
func genRandStr(length int) (string, error) {
	buffer := make([]byte, length)

	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(buffer)[:length], nil
}

type Peer struct {
	ip   string
	port uint16
}

func (p Peer) String() string {
	return fmt.Sprintf("%s:%d", p.ip, p.port)
}

// parsePeers parses the peers info from the tracker response.
// The peers info is a string of 6 bytes for each peer.
// The first 4 bytes represent the IP address of the peer.
// The last 2 bytes represent the port of the peer.
func parsePeers(peersInfo string) ([]Peer, error) {
	peers := make([]Peer, 0)

	for i := 0; i < len(peersInfo); i += 6 {
		ip := fmt.Sprintf("%d.%d.%d.%d", peersInfo[i], peersInfo[i+1], peersInfo[i+2], peersInfo[i+3])
		// Port is a big-endian 16-bit integer
		// The first byte is shifted 8 bits to the left and the second byte is added
		port := int16(peersInfo[i+4])<<8 + int16(peersInfo[i+5])

		peers = append(peers, Peer{ip, uint16(port)})
	}

	return peers, nil
}

// discoverPeers sends a request to the tracker to discover peers.
// The returned response is a list of peer IP addresses and ports.
func discoverPeers(filename string) (peersInfo []Peer, err error) {
	mf, err := parseMetaFile(filename)
	if err != nil {
		return
	}

	body, err := requestTracker(mf)
	if err != nil {
		return
	}

	trackerInfo, err := decodeBencode(string(body))
	if err != nil {
		return
	}

	peersInfoBencoded, ok := trackerInfo.(map[string]any)["peers"].(string)
	if !ok {
		err = fmt.Errorf("Invalid peers info")
		return
	}

	peersInfo, err = parsePeers(peersInfoBencoded)
	if err != nil {
		return
	}

	return
}

// requestTracker sends a request to the tracker to discover peers.
// The returned response is a bencoded dictionary with the peers info.
func requestTracker(mf *MetaFile) ([]byte, error) {
	infoHash, err := mf.Info.Sha1Sum()
	if err != nil {
		return nil, err
	}

	peerId, err := genRandStr(20)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Add("info_hash", infoHash)
	query.Add("peer_id", peerId)
	query.Add("port", "6881")
	query.Add("uploaded", "0")
	query.Add("downloaded", "0")
	query.Add("left", strconv.Itoa(mf.Info.Length))
	query.Add("compact", "1")

	url := mf.Announce + "?" + query.Encode()

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))

	case "info":
		filename := os.Args[2]

		mf, err := parseMetaFile(filename)
		if err != nil {
			fmt.Println(err)
			return
		}

		infoHash, err := mf.Info.Sha1Sum()
		if err != nil {
			fmt.Println(err)
			return
		}

		pieceHashes := mf.Info.PieceHashes()

		fmt.Printf("Tracker URL: %v\n", mf.Announce)
		fmt.Printf("Length: %v\n", mf.Info.Length)
		fmt.Printf("Info Hash: %x\n", infoHash)
		fmt.Printf("Piece Length: %v\n", mf.Info.PieceLength)
		fmt.Printf("Piece Hashes:\n%v\n", strings.Join(pieceHashes, "\n"))

	case "peers":
		filename := os.Args[2]

		peersInfo, err := discoverPeers(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Pretty print peers
		var peersInfoStr string
		for _, peer := range peersInfo {
			peersInfoStr = fmt.Sprintf("%s%s\n", peersInfoStr, peer)
		}

		fmt.Printf("Peers Discovered:\n%v", peersInfoStr)

	default:
		fmt.Println("" + command)
		os.Exit(1)

	}
}
