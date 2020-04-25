package peer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

const (
	// PeerID is our peer ID
	PeerID              = "-LO00177770000077777"
	protocolStr         = "BitTorrent protocol"
	protocolLen   uint8 = 19
	bufferSize          = 1024
	handshakeSize       = 68
)

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
}

// A block is downloaded by the client when the client is interested in a peer,
// and that peer is not choking the client. A block is uploaded by a client when
// the client is not choking a peer, and that peer is interested in the client.

// It is important for the client to keep its peers informed as to whether or not
// it is interested in them. This state information should be kept up-to-date with
// each peer even when the client is choked. This will allow peers to know if the
// client will begin downloading when it is unchoked (and vice-versa).
type peerConnection struct {
	Socket         net.Conn
	File           File
	AmChoking      bool
	AmInterested   bool
	PeerChoking    bool
	PeerInterested bool
}

type handshake struct {
	PStrLen  uint8
	PStr     [19]byte
	Reserved uint64
	InfoHash [20]byte
	PeerID   [20]byte
}

// <length prefix><message ID><payload>
type message struct {
	Length  uint32
	ID      uint8
	Payload []byte
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}

// Download begins the Peer Wire Protocol with each peer over TCP
func Download(file File) {
	for i := 0; i < len(file.Peers); i++ {
		conn, err := net.DialTimeout("tcp", file.Peers[i].String(), 3*time.Second)
		if err != nil {
			continue
		}
		p := newPeerConnection(file, conn)
		err = p.beginProtocol()
		if err != nil {
			conn.Close()
			continue
		}
	}
}

func newPeerConnection(file File, socket net.Conn) (p *peerConnection) {
	return &peerConnection{
		Socket:         socket,
		File:           file,
		AmChoking:      true,
		AmInterested:   false,
		PeerChoking:    true,
		PeerInterested: false,
	}
}

func (p *peerConnection) beginProtocol() error {
	err := p.handshake()
	if err != nil {
		return err
	}
	message, err := p.readMessage()
	fmt.Println(message)
	return nil
}

func (p *peerConnection) handshake() error {
	payload := handshake{
		PStrLen:  protocolLen,
		Reserved: 0,
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
	m.Payload = make([]byte, m.Length)
	err = p.read(&m.Payload, int(m.Length))
	if err != nil {
		return m, err
	}
	return m, nil
}

func (p *peerConnection) write(payload interface{}) error {
	writeBuffer := bytes.NewBuffer(make([]byte, 0, bufferSize))
	binary.Write(writeBuffer, binary.BigEndian, payload)
	_, err := p.Socket.Write(writeBuffer.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func (p *peerConnection) read(response interface{}, size int) error {
	readData := make([]byte, size)
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
