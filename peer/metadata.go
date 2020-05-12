package peer

import (
	"errors"
	"fmt"
)

type fileInfo struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

// TorrentInfo represents the info section of a torrent file (the metadata)
type TorrentInfo struct {
	Pieces                string     `bencode:"pieces"`
	PieceLength           int        `bencode:"piece length"`
	Length                int        `bencode:"length"`
	Name                  string     `bencode:"name"`
	Files                 []fileInfo `bencode:"files"`
	PiecesList            [][20]byte
	MetadataSize          int
	MovieSize             int
	MoviePath             []string
	MovieStartByte        int
	MovieEndByte          int
	MovieStartPiece       int
	MovieEndPiece         int
	MovieStartPieceOffset int
	MovieEndPieceOffset   int
}

// PrepareForDownload rearranges the metadata to allow for easier calculations when downloading
func (t *TorrentInfo) PrepareForDownload() error {
	var err error = nil
	err = t.splitPieceHashes()
	t.setMovieSize()
	t.setMovieBounds()
	return err
}

func (t *TorrentInfo) splitPieceHashes() error {
	hashLen := 20 // Length of SHA-1 hash
	buf := []byte(t.Pieces)
	if len(buf)%hashLen != 0 {
		err := fmt.Errorf("Received malformed pieces of length %d", len(buf))
		return err
	}
	numHashes := len(buf) / hashLen
	hashes := make([][20]byte, numHashes)

	for i := 0; i < numHashes; i++ {
		copy(hashes[i][:], buf[i*hashLen:(i+1)*hashLen])
	}

	t.PiecesList = hashes
	return nil
}

func (t *TorrentInfo) setMovieSize() {
	movieSize := 0
	var moviePath []string
	for _, file := range t.Files {
		if file.Length > movieSize {
			movieSize = file.Length
			moviePath = file.Path
		}
	}
	t.MovieSize = movieSize
	t.MoviePath = moviePath
}

func (t *TorrentInfo) setMovieBounds() {
	bytesBeforeMovie := 0
	for _, file := range t.Files {
		if file.Length != t.MovieSize {
			bytesBeforeMovie += file.Length
		} else {
			break
		}
	}
	t.MovieStartByte = bytesBeforeMovie
	t.MovieEndByte = bytesBeforeMovie + t.MovieSize
	t.MovieStartPiece = t.MovieStartByte / t.PieceLength
	t.MovieEndPiece = t.MovieEndByte / t.PieceLength
	t.MovieStartPieceOffset = t.MovieStartByte % t.PieceLength
	t.MovieEndPieceOffset = t.MovieEndByte % t.PieceLength
}

func (t *TorrentInfo) calculatePieceSize(index int) (int, error) {
	if index < t.MovieStartPiece {
		return 0, errors.New("Index cannot be smaller than the movie start index")
	}
	bytesLeft := t.MovieSize - ((index - t.MovieStartPiece) * t.PieceLength)
	if t.PieceLength < bytesLeft {
		return t.PieceLength, nil
	}
	return bytesLeft, nil
}

func numPiecesInMovie(t *TorrentInfo) int {
	return t.MovieEndPiece - t.MovieStartPiece
}
