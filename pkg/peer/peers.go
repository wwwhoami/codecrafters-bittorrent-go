package peer

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/bencode"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/util"
)

type Peer struct {
	ip   string
	port uint16
}

func (p Peer) String() string {
	return fmt.Sprintf("%s:%d", p.ip, p.port)
}

func NewPeerFromAddr(addr string) (*Peer, error) {
	peerStr := strings.Split(addr, ":")

	peerIp := peerStr[0]
	if net.ParseIP(peerIp) == nil {
		return nil, fmt.Errorf("invalid IP address: %v", peerIp)
	}

	peerPort, err := strconv.Atoi(peerStr[1])
	if err != nil {
		return nil, err
	} else if peerPort < 0 || peerPort > 65535 {
		return nil, fmt.Errorf("invalid port: %v", peerPort)
	}

	return &Peer{peerIp, uint16(peerPort)}, nil
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

// DiscoverPeers sends a request to the tracker to discover peers.
// Announce is the URL of the tracker, infoHash is the SHA1 hash of the torrent file,
// and infoLength is the length of the file.
// The returned response is a list of peer IP addresses and ports.
func DiscoverPeers(announce, infoHash string, infoLength int) (peers []Peer, err error) {
	body, err := requestTracker(announce, infoHash, infoLength)
	if err != nil {
		return
	}

	trackerInfo, err := bencode.DecodeBytes((body))
	if err != nil {
		return
	}

	peersInfoBencoded, ok := trackerInfo.(map[string]any)["peers"].(string)
	if !ok {
		err = fmt.Errorf("invalid peers info")
		return
	}

	peers, err = parsePeers(peersInfoBencoded)
	if err != nil {
		return
	}

	return
}

// requestTracker sends a request to the tracker to discover peers.
// The request includes the info hash and the file length.
// The returned response is a bencoded dictionary with the peers info.
func requestTracker(announce, infoHash string, fileLength int) ([]byte, error) {
	peerId, err := util.GenRandStr(20)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Add("info_hash", infoHash)
	query.Add("peer_id", peerId)
	query.Add("port", "6881")
	query.Add("uploaded", "0")
	query.Add("downloaded", "0")
	query.Add("left", strconv.Itoa(fileLength))
	query.Add("compact", "1")

	url := announce + "?" + query.Encode()

	fmt.Printf("Requesting tracker: %v\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
