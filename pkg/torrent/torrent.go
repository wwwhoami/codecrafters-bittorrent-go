package torrent

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/codecrafters-io/bittorrent-starter-go/pkg/metainfo"
	"github.com/codecrafters-io/bittorrent-starter-go/pkg/peer"
)

type Torrent struct {
	mf        *metainfo.MetaFile
	workQueue chan *PieceWork
	peerConns []*peer.PeerConn
}

func NewTorrent(mf *metainfo.MetaFile) (*Torrent, error) {
	peersInfo, err := peer.DiscoverPeers(mf.Announce, mf.Info.Hash, mf.Info.Length)
	if err != nil {
		return nil, fmt.Errorf("failed to discover peers: %v", err)
	}

	t := &Torrent{
		mf,
		make(chan *PieceWork, len(mf.Info.PieceHashes)),
		make([]*peer.PeerConn, 0, len(peersInfo)),
	}

	if err := t.connectPeers(peersInfo); err != nil {
		return nil, fmt.Errorf("failed to connect to peers: %v", err)
	}

	return t, nil
}

func (t *Torrent) addPeerConn(pc *peer.PeerConn) {
	t.peerConns = append(t.peerConns, pc)
}

func (t *Torrent) addPiece(p *PieceWork) {
	t.workQueue <- p
}

const PieceDownloadRetries = 5

// connectPeers connects to the peers in the given list. It creates a PeerConn
// for each peer and performs a handshake with the peer. If the handshake is
// successful, it adds the PeerConn to the Torrent's peerConns list.
// If the handshake fails, it returns an error.
func (t *Torrent) connectPeers(peersInfo []peer.Peer) error {
	for _, p := range peersInfo {
		pc, err := peer.NewPeerConn(p, t.mf.Info.Hash)
		if err != nil {
			return fmt.Errorf("failed to create peer %v connection: %v", p, err)
		}

		t.addPeerConn(pc)
	}

	return nil
}

// DownloadFile downloads the file from the torrent to the given output file.
// It downloads the pieces concurrently from the available peers. If a piece
// download fails, it retries a few times before giving up. If all pieces are
// downloaded successfully, it writes the pieces to the output file.
func (t *Torrent) DownloadFile(outFilename string) (err error) {
	startTime := time.Now()

	var wg sync.WaitGroup
	waitCh := make(chan struct{})

	pieceHashes := t.mf.Info.PieceHashes

	wg.Add(len(pieceHashes))

	for i, pieceHash := range pieceHashes {
		pieceWork := NewPieceWork(&Piece{hash: pieceHash, idx: i})
		t.addPiece(pieceWork)
	}

	errCh := make(chan error, len(pieceHashes))

	pieces := make([]*Piece, len(pieceHashes))

	// Worker function downloads pieces from peers
	worker := func(pc *peer.PeerConn) {
		fmt.Printf("Goroutine for Peer %v started\n", pc.Peer)

		if err := pc.PreDownload(); err != nil {
			errCh <- fmt.Errorf("failed to prepare download: %v", err)
			return
		}

		for pieceWork := range t.workQueue {
			piece := pieceWork.piece

			log.Printf("Downloading Piece %d from Peer %v\n", piece.idx, pc.Peer)

			piece.data, err = pc.DownloadPiece(t.mf, piece.idx)
			if err != nil {
				log.Printf("Attempting to download piece %d from Peer %v failed: %v\n", piece.idx, pc.Peer, err)

				if pieceWork.retries < PieceDownloadRetries {
					log.Printf("Adding piece %d back to work queue\n", piece.idx)
					pieceWork.retries++
					t.workQueue <- pieceWork
					continue
				}

				errCh <- fmt.Errorf("failed to download piece: %v", err)
				return
			}

			pieces[piece.idx] = piece

			wg.Done()
		}

		log.Printf("Goroutine for Peer %v finished\n", pc.Peer)
	}

	// Initialize worker for each peer
	for _, pc := range t.peerConns {
		go worker(pc)
	}

	go func() {
		wg.Wait()
		close(waitCh)
	}()

	// Wait for all pieces to be downloaded
	// or for an error to occur
L:
	for {
		select {
		case <-waitCh:
			log.Println("All pieces downloaded with no errors")
			break L
		case err = <-errCh:
			return
		}
	}

	close(t.workQueue)
	close(errCh)

	if err = writePiecesToOut(outFilename, pieces); err != nil {
		err = fmt.Errorf("failed to write to output file: %v", err)
		return
	}

	log.Printf("File %s successfully downloaded in %.3fs\n", outFilename, time.Since(startTime).Seconds())

	return
}

// Close closes all peer connections.
func (t *Torrent) Close() {
	for _, pc := range t.peerConns {
		pc.Close()
	}
}

// Piece represents a piece of the file to be downloaded.
type Piece struct {
	hash string
	data []byte
	idx  int
}

// PieceWork represents a piece of work to be done by a worker.
type PieceWork struct {
	piece   *Piece
	retries int
}

func NewPieceWork(piece *Piece) *PieceWork {
	return &PieceWork{piece, 0}
}

// writePiecesToOut writes the pieces to the output file
// in the order of the piece indices.
func writePiecesToOut(outFilename string, pieces []*Piece) error {
	outFile, err := os.Create(outFilename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	for _, piece := range pieces {
		if piece != nil {
			_, err = outFile.Write(piece.data)
			if err != nil {
				return fmt.Errorf("failed to write piece data to file: %v", err)
			}
		}
	}

	return nil
}
