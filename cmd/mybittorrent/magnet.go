package main

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// parseMagnetLink parses the magnet link and returns the info hash(binary encoded), filename and tracker URL.
// The magnet link should be in the v1 magnet format: "magnet:?xt=urn:btih:<info_hash>&dn=<filename>&tr=<tracker_url>"
func parseMagnetLink(magnetLink string) (infoHash, filename, trackerURL string, err error) {
	if !strings.HasPrefix(magnetLink, "magnet:?") {
		err = fmt.Errorf("invalid magnet link: %v", magnetLink)
		return
	}

	magnetLink = strings.TrimPrefix(magnetLink, "magnet:?")

	parts := strings.Split(magnetLink, "&")

	for _, part := range parts {
		if strings.HasPrefix(part, "xt=urn:btih:") {
			infoHash = strings.TrimPrefix(part, "xt=urn:btih:")
		} else if strings.HasPrefix(part, "dn=") {
			filename = strings.TrimPrefix(part, "dn=")
		} else if strings.HasPrefix(part, "tr=") {
			trackerURL = strings.TrimPrefix(part, "tr=")
		}
	}

	if infoHash == "" || filename == "" || trackerURL == "" {
		err = fmt.Errorf("invalid magnet link: %v", magnetLink)
		return
	}

	// Unescape the tracker URL
	trackerURL, err = url.QueryUnescape(trackerURL)
	if err != nil {
		err = fmt.Errorf("failed to unescape tracker URL: %v", err)
		return
	}

	// Convert the info hash from hex to binary
	infoHashHex, err := hex.DecodeString(infoHash)
	if err != nil {
		err = fmt.Errorf("failed to decode info hash: %v", err)
		return
	}

	infoHash = string(infoHashHex)

	return
}
