package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

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
		err = fmt.Errorf("invalid peers info")
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
