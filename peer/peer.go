package peer

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"
)

const (
	// PeerID is our peer ID
	PeerID                         = "-LO00177770000077777"
	protocolStr                    = "BitTorrent protocol"
	protocolLen      uint8         = 19
	bufferSize                     = 1024
	handshakeSize                  = 68
	timeoutDuration  time.Duration = 3 * time.Second
	maxRequestLength               = 16384 //16KiB
	maxBacklog                     = 5
)

const (
	msgChoke         uint8 = 0
	msgUnchoke       uint8 = 1
	msgInterested    uint8 = 2
	msgNotInterested uint8 = 3
	msgHave          uint8 = 4
	msgBitfield      uint8 = 5
	msgRequest       uint8 = 6
	msgPiece         uint8 = 7
	msgCancel        uint8 = 8
	msgPort          uint8 = 9
	msgExtended      uint8 = 20
)

var haveMetadata bool = false

// Peer is the IP and Port of a host in the swarm
type Peer struct {
	IP   net.IP
	Port uint16
}

// File represents the file we wish to download
type File struct {
	InfoHash [20]byte
	Name     string
	Peers    []Peer
	Metadata *TorrentInfo
}

// A block is downloaded by the client when the client is interested in a peer,
// and that peer is not choking the client. A block is uploaded by a client when
// the client is not choking a peer, and that peer is interested in the client.
type peerConnection struct {
	Socket               net.Conn
	File                 *File
	AmChoking            bool
	AmInterested         bool
	PeerChoking          bool
	PeerInterested       bool
	ExtMetadata          uint8
	CurrentMetadataPiece int
	MetadataSize         int
	MetadataBuff         *bytes.Buffer
	Done                 bool
	Bitfield             []byte
	CurrentPiece         *pieceState
}

type pieceState struct {
	Index      int
	Downloaded int
	Requested  int
	Backlog    int
	Buff       []byte
}

type handshake struct {
	PStrLen  uint8
	PStr     [19]byte
	Reserved [8]byte
	InfoHash [20]byte
	PeerID   [20]byte
}

type message struct {
	Length  uint32
	ID      uint8
	Payload []byte
}

type inputPiece struct {
	Index  int
	Hash   [20]byte
	Length int
}

type outputPiece struct {
	Index int
	Buff  []byte
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}

// DownloadMovie only downloads the largest file (the movie) from the torrent
func DownloadMovie(file *File) error {
	inputPieces := make(chan *inputPiece, file.Metadata.Movie.NumPieces)
	outputPieces := make(chan *outputPiece)
	for i := file.Metadata.Movie.StartPiece; i < file.Metadata.Movie.EndPiece; i++ {
		length, err := file.Metadata.calculatePieceSize(i)
		if err != nil {
			return err
		}
		inputPieces <- &inputPiece{i, file.Metadata.PiecesList[i], length}
	}

	for i := 0; i < len(file.Peers); i++ {
		go startDownloadWorker(file, file.Peers[i], inputPieces, outputPieces)
	}

	f, err := os.Create("movies/" + file.Metadata.Movie.Path[len(file.Metadata.Movie.Path)-1])
	if err != nil {
		return err
	}
	for i := 0; i < file.Metadata.Movie.NumPieces; i++ {
		donePiece := <-outputPieces
		percentDone := ((float32(i) + 1) / float32(file.Metadata.Movie.NumPieces)) * 100
		f.WriteAt(donePiece.Buff, int64(donePiece.Index*file.Metadata.PieceLength))
		fmt.Printf("Downloaded piece at index %d, of length: %d \n", donePiece.Index, len(donePiece.Buff))
		fmt.Printf("Currently downloading from %d peers \n", runtime.NumGoroutine()-1)
		fmt.Printf("Percent done: %0.2f %% \n", percentDone)
	}

	return nil
}

func startDownloadWorker(file *File, peer Peer, inputPieces chan *inputPiece, outputPieces chan *outputPiece) {
	conn, err := net.DialTimeout("tcp", peer.String(), 6*time.Second)
	if err != nil {
		return
	}
	p := newPeerConnection(file, conn)
	err = p.peerWireProtocol()
	if err != nil {
		conn.Close()
		return
	}
	beginDownload(p, inputPieces, outputPieces)
}

func beginDownload(p *peerConnection, inputPieces chan *inputPiece, outputPieces chan *outputPiece) {
	for piece := range inputPieces {
		if !p.hasPiece(piece.Index) {
			inputPieces <- piece
			continue
		}
		buf, err := p.attemptDownloadPiece(piece)
		if err != nil {
			log.Println("Failed to download piece", err)
			inputPieces <- piece // Put piece back on the queue
			p.Socket.Close()
			return
		}
		err = validatePiece(piece, buf)
		if err != nil {
			log.Printf("Piece #%d failed integrity check\n", piece.Index)
			inputPieces <- piece // Put piece back on the queue
			continue
		}
		outputPieces <- &outputPiece{piece.Index, buf}
	}
}

func validatePiece(piece *inputPiece, downloadedData []byte) error {
	hash := sha1.Sum(downloadedData)
	if !bytes.Equal(hash[:], piece.Hash[:]) {
		return fmt.Errorf("Index %d failed integrity check", piece.Index)
	}
	return nil
}

func newPeerConnection(file *File, socket net.Conn) (p *peerConnection) {
	return &peerConnection{
		Socket:               socket,
		File:                 file,
		AmChoking:            true,
		AmInterested:         false,
		PeerChoking:          true,
		PeerInterested:       false,
		CurrentMetadataPiece: 0,
		MetadataSize:         0,
		MetadataBuff:         &bytes.Buffer{},
		Bitfield:             newBitfield(file),
	}
}

func newBitfield(file *File) []byte {
	piecesPerByte := 8
	hashLength := 20
	hashPiecesLength := 0
	if file.Metadata != nil {
		hashPiecesLength = len(file.Metadata.Pieces)
	}
	numPieces := hashPiecesLength / hashLength
	return make([]byte, numPieces/piecesPerByte)
}

func (p *peerConnection) peerWireProtocol() error {
	err := p.handshake()
	if err != nil {
		return err
	}
	err = p.sendInterested()
	if err != nil {
		return err
	}
	for p.AmChoking {
		message, err := p.readMessage()
		if err != nil {
			return err
		}
		err = p.handleMessage(message)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *peerConnection) handshake() error {
	reserved := [8]byte{0, 0, 0, 0, 0, 0, 0, 0}
	reserved[5] |= 0x10
	payload := handshake{
		PStrLen:  protocolLen,
		Reserved: reserved,
		InfoHash: p.File.InfoHash,
	}
	var response handshake
	copy(payload.PStr[:19], protocolStr)
	copy(payload.PeerID[:20], PeerID)
	p.write(&payload)
	err := p.read(&response, handshakeSize)
	if !bytes.Equal(payload.InfoHash[:], response.InfoHash[:]) {
		return fmt.Errorf("Expected infohash %x but got %x", payload.InfoHash, response.InfoHash)
	}

	if err != nil {
		return err
	}
	return nil
}

func (p *peerConnection) handleMessage(m message) error {
	switch m.ID {
	case msgChoke:
		p.AmChoking = true
	case msgUnchoke:
		p.AmChoking = false
	case msgInterested:
		p.PeerInterested = true
	case msgNotInterested:
		p.PeerInterested = false
	case msgHave:
		index := int(binary.BigEndian.Uint32(m.Payload))
		p.setPiece(index)
	case msgBitfield:
		p.Bitfield = m.Payload
	case msgRequest:
		payload := 0
		p.write(&payload)
	case msgPiece:
		err := p.handlePiece(m)
		if err != nil {
			return err
		}
	case msgCancel:
		payload := 0
		p.write(&payload)
	case msgPort:
		payload := 0
		p.write(&payload)
	}
	return nil
}

func (p *peerConnection) setPiece(index int) {
	bitInByte := index % 8
	byteIndex := index / 8
	// start at the beginning of the byte, then shift right
	var newBit uint8 = 128 >> bitInByte
	p.Bitfield[byteIndex] |= newBit
}

func (p *peerConnection) hasPiece(index int) bool {
	bitInByte := index % 8
	byteIndex := index / 8
	if byteIndex < 0 || byteIndex >= len(p.Bitfield) {
		return false
	}
	return p.Bitfield[byteIndex]>>(7-bitInByte)&1 != 0
}

func (p *peerConnection) attemptDownloadPiece(piece *inputPiece) ([]byte, error) {
	state := pieceState{
		Index: piece.Index,
		Buff:  make([]byte, piece.Length),
	}
	p.CurrentPiece = &state
	// Setting a deadline helps get unresponsive peers unstuck.
	// 30 seconds is more than enough time to download a 262 KB piece
	// p.Socket.SetDeadline(time.Now().Add(30 * time.Second))
	// defer p.Socket.SetDeadline(time.Time{}) // Disable the deadline

	for state.Downloaded < piece.Length {
		// If unchoked, send requests until we have enough unfulfilled requests
		if !p.AmChoking {
			for state.Backlog < maxBacklog && state.Requested < piece.Length {
				blockSize := maxRequestLength
				leftToRequest := piece.Length - state.Requested
				// Last block might be shorter than the typical block
				if leftToRequest < blockSize {
					blockSize = leftToRequest
				}

				err := p.requestPiece(piece.Index, state.Requested, blockSize)
				if err != nil {
					return nil, err
				}
				state.Backlog++
				state.Requested += blockSize
			}
		}

		message, err := p.readMessage()
		if err != nil {
			return nil, err
		}
		p.handleMessage(message)
	}

	return state.Buff, nil
}

func (p *peerConnection) requestPiece(index int, begin int, length int) error {
	m := message{
		Length: 13,
		ID:     msgRequest,
	}
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))
	m.Payload = payload
	err := p.writeMessage(m)
	if err != nil {
		return err
	}
	return nil
}

func (p *peerConnection) handlePiece(m message) error {
	if m.ID != msgPiece {
		return fmt.Errorf("Expected PIECE (ID %d), got ID %d", msgPiece, m.ID)
	}
	if len(m.Payload) < 8 {
		return fmt.Errorf("Payload too short. %d < 8", len(m.Payload))
	}
	parsedIndex := int(binary.BigEndian.Uint32(m.Payload[0:4]))
	if parsedIndex != p.CurrentPiece.Index {
		return fmt.Errorf("Expected index %d, got %d", p.CurrentPiece.Index, parsedIndex)
	}
	begin := int(binary.BigEndian.Uint32(m.Payload[4:8]))
	if begin >= len(p.CurrentPiece.Buff) {
		return fmt.Errorf("Begin offset too high. %d >= %d", begin, len(p.CurrentPiece.Buff))
	}
	data := m.Payload[8:]
	if begin+len(data) > len(p.CurrentPiece.Buff) {
		return fmt.Errorf("Data too long [%d] for offset %d with length %d", len(data), begin, len(p.CurrentPiece.Buff))
	}
	copy(p.CurrentPiece.Buff[begin:], data)
	bytesWritten := len(data)
	p.CurrentPiece.Downloaded += bytesWritten
	p.CurrentPiece.Backlog--
	return nil
}

func (p *peerConnection) sendInterested() error {
	m := message{
		Length: 1,
		ID:     msgInterested,
	}
	return p.writeMessage(m)
}

// <len><id><payload>
func (p *peerConnection) writeMessage(m message) error {
	writeBuffer := bytes.NewBuffer(make([]byte, 0, bufferSize))
	var length uint32 = uint32(len(m.Payload) + 1) // ID + payload
	err := binary.Write(writeBuffer, binary.BigEndian, &length)
	if err != nil {
		return err
	}
	err = binary.Write(writeBuffer, binary.BigEndian, &m.ID)
	if err != nil {
		return err
	}
	err = binary.Write(writeBuffer, binary.BigEndian, &m.Payload)
	if err != nil {
		return err
	}
	_, err = p.Socket.Write(writeBuffer.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func (p *peerConnection) readMessage() (message, error) {
	var m message
	err := p.read(&m.Length, 4)
	if err != nil {
		return m, err
	}
	err = p.read(&m.ID, 1)
	if err != nil {
		return m, err
	}
	m.Payload = make([]byte, m.Length-1)
	err = p.read(&m.Payload, int(m.Length-1))
	if err != nil {
		return m, err
	}
	return m, nil
}

func (p *peerConnection) write(payload interface{}) error {
	writeBuffer := bytes.NewBuffer(make([]byte, 0, bufferSize))
	err := binary.Write(writeBuffer, binary.BigEndian, payload)
	if err != nil {
		return err
	}
	_, err = p.Socket.Write(writeBuffer.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func (p *peerConnection) read(response interface{}, size int) error {
	readData := make([]byte, size)
	p.Socket.SetReadDeadline(time.Now().Add(timeoutDuration))
	bytesRead, err := io.ReadFull(p.Socket, readData)
	if err != nil {
		return err
	}
	readBuffer := bytes.NewBuffer(readData[:bytesRead])
	err = binary.Read(readBuffer, binary.BigEndian, response)
	if err != nil {
		return err
	}
	return nil
}
