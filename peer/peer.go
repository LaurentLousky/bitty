package peer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
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
	index  int
	hash   [20]byte
	length int
}

type outputPiece struct {
	index int
	buf   []byte
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}

// Download begins the Peer Wire Protocol with each peer over TCP
func Download(file *File) error {
	inputPieces := make(chan *inputPiece)
	// outputPieces := make(chan *outputPiece)
	for i := file.Metadata.MovieStartPiece; i < len(file.Metadata.PiecesList); i++ {
		length, err := file.Metadata.calculatePieceSize(i)
		if err != nil {
			return err
		}
		inputPieces <- &inputPiece{i, file.Metadata.PiecesList[i], length}
	}
	for i := 0; i < len(file.Peers); i++ {
		conn, err := net.DialTimeout("tcp", file.Peers[i].String(), 3*time.Second)
		if err != nil {
			continue
		}
		p := newPeerConnection(file, conn)
		err = p.peerWireProtocol()
		if err != nil {
			conn.Close()
			continue
		}
	}
	return errors.New("Some error here")
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

	done := false
	for done == false {
		message, err := p.readMessage()
		fmt.Println(err)
		p.handleMessage(message)
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
		index := binary.BigEndian.Uint32(m.Payload)
		p.setPiece(index)
		if p.AmInterested != true {
			return p.sendInterested()
		}
	case msgBitfield:
		p.Bitfield = m.Payload
		if p.AmInterested != true {
			return p.sendInterested()
		}
	case msgRequest:
		payload := 0
		p.write(&payload)
	case msgPiece:
		p.downloadPiece()
	case msgCancel:
		payload := 0
		p.write(&payload)
	case msgPort:
		payload := 0
		p.write(&payload)
	default:
		return errors.New("Cannot understand message ID")
	}
	return nil
}

func (p *peerConnection) setPiece(index uint32) {
	var bitInByte uint32 = index % 8
	var byteIndex uint32 = index / 8
	// start at the beginning of the byte, then shift right
	var newBit uint8 = 128 >> bitInByte
	p.Bitfield[byteIndex] |= newBit
}

func (p *peerConnection) downloadPiece() {
	println("Beginnning to download peice")
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
	bytesWritten, err := p.Socket.Write(writeBuffer.Bytes())
	fmt.Printf("Bytes written to socket: %d \n", bytesWritten)
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
	bytesWritten, err := p.Socket.Write(writeBuffer.Bytes())
	fmt.Printf("Bytes written to socket: %d \n", bytesWritten)
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
