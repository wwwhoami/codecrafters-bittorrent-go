package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type Torrent struct {
	mf        *MetaFile
	workQueue chan *PieceWork
	peerConns []*PeerConn
}

func NewTorrent(mf *MetaFile) (*Torrent, error) {
	workQueueLen := len(mf.Info.PieceHashes)

	peersInfo, err := discoverPeers(mf)
	if err != nil {
		return nil, fmt.Errorf("failed to discover peers: %v", err)
	}

	peersLen := len(peersInfo)

	return &Torrent{mf, make(chan *PieceWork, workQueueLen), make([]*PeerConn, 0, peersLen)}, nil
}

func (t *Torrent) AddPeerConn(pc *PeerConn) {
	t.peerConns = append(t.peerConns, pc)
}

func (t *Torrent) AddPiece(p *PieceWork) {
	t.workQueue <- p
}

func (t *Torrent) NextPiece() *PieceWork {
	return <-t.workQueue
}

type Piece struct {
	hash string
	data []byte
	idx  int
}

type PieceWork struct {
	piece   *Piece
	retries int
}

func NewPieceWork(piece *Piece) *PieceWork {
	return &PieceWork{piece, 0}
}

const PieceDownloadRetries = 5

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
		t.AddPiece(pieceWork)
	}

	errCh := make(chan error, len(pieceHashes))

	pieces := make([]*Piece, len(pieceHashes))

	// Worker function downloads pieces from peers
	worker := func(pc *PeerConn) {
		fmt.Printf("Goroutine for Peer %v started\n", pc.peer)

		if err := pc.PreDownload(); err != nil {
			errCh <- fmt.Errorf("failed to prepare download: %v", err)
			return
		}

		for pieceWork := range t.workQueue {
			piece := pieceWork.piece

			log.Printf("Downloading Piece %d from Peer %v\n", piece.idx, pc.peer)

			piece.data, err = pc.DownloadPiece(t.mf, piece.idx)
			if err != nil {
				log.Printf("Attempting to download piece %d from Peer %v failed: %v\n", piece.idx, pc.peer, err)

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

		log.Printf("Goroutine for Peer %v finished\n", pc.peer)
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
