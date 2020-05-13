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
	Pieces       string     `bencode:"pieces"`
	PieceLength  int        `bencode:"piece length"`
	Length       int        `bencode:"length"`
	Name         string     `bencode:"name"`
	Files        []fileInfo `bencode:"files"`
	PiecesList   [][20]byte
	MetadataSize int
	Movie        movie
}

type movie struct {
	Size             int
	Path             []string
	StartByte        int
	EndByte          int
	StartPiece       int
	EndPiece         int
	StartPieceOffset int
	EndPieceOffset   int
	NumPieces        int
}

// PrepareForDownload rearranges the metadata to allow for easier calculations when downloading
func (t *TorrentInfo) PrepareForDownload() error {
	var err error = nil
	err = t.splitPieceHashes()
	t.setMovieSize()
	t.setMovieBounds()
	t.seMovietNumPieces()
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
	t.Movie.Size = movieSize
	t.Movie.Path = moviePath
}

func (t *TorrentInfo) setMovieBounds() {
	bytesBeforeMovie := 0
	for _, file := range t.Files {
		if file.Length != t.Movie.Size {
			bytesBeforeMovie += file.Length
		} else {
			break
		}
	}
	t.Movie.StartByte = bytesBeforeMovie
	t.Movie.EndByte = bytesBeforeMovie + t.Movie.Size
	t.Movie.StartPiece = t.Movie.StartByte / t.PieceLength
	t.Movie.EndPiece = t.Movie.EndByte / t.PieceLength
	t.Movie.StartPieceOffset = t.Movie.StartByte % t.PieceLength
	t.Movie.EndPieceOffset = t.Movie.EndByte % t.PieceLength
}

func (t *TorrentInfo) calculatePieceSize(index int) (int, error) {
	if index < t.Movie.StartPiece {
		return 0, errors.New("Index cannot be smaller than the movie start index")
	}
	bytesLeft := t.Movie.Size - ((index - t.Movie.StartPiece) * t.PieceLength)
	if t.PieceLength < bytesLeft {
		return t.PieceLength, nil
	}
	return bytesLeft, nil
}

func (t *TorrentInfo) seMovietNumPieces() {
	t.Movie.NumPieces = t.Movie.EndPiece - t.Movie.StartPiece
}
