package magneturi

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"time"

	"github.com/laurentlousky/stream/peer"
)

const (
	port                    uint16 = 6888
	bufferSize                     = 2048
	connectionID                   = 0x41727101980
	actionConnect                  = 0
	actionAnnounce                 = 1
	actionError                    = 3
	eventNone                      = 0
	eventCompleted                 = 1
	eventStarted                   = 2
	eventStopped                   = 3
	announceMinResponseSize        = 20
	peerSize                       = 6
	maxRequestAttempts             = 2
)

type connectionRequest struct {
	ConnectionID  int64
	Action        int32
	TransactionID int32
}

type connectionResponse struct {
	Action        int32
	TransactionID int32
	ConnectionID  int64
}

type announceRequest struct {
	ConnectionID  int64
	Action        int32
	TransactionID int32
	InfoHash      [20]byte
	PeerID        [20]byte
	Downloaded    int64
	Left          int64
	Uploaded      int64
	Event         int32
	IP            uint32
	Key           uint32
	NumWant       int32
	Port          uint16
}

type announceResponse struct {
	header announceResponseHeader
	body   announceResponseBody
}
type announceResponseHeader struct {
	Action        int32
	TransactionID int32
	Interval      int32
	Leechers      int32
	Seeders       int32
}

type announceResponseBody struct {
	Peers []peer.Peer
}

type client struct {
	Socket  net.Conn
	Tracker string
}

func newTransactionID() int32 {
	return int32(rand.Uint32())
}

func (m *MagnetURI) requestPeers() ([]peer.Peer, error) {
	var c client
	var announceResp announceResponse
	for _, tracker := range m.Trackers {
		c.Tracker = tracker
		connectResp, err := c.connect()
		if err != nil {
			continue
		}
		announceReq := m.newAnnounceRequest(connectResp)
		announceResp, err := c.announce(announceReq)
		if err != nil {
			continue
		}
		if announceResp.header.Action == actionAnnounce {
			fmt.Printf("Announced successfully to: %s \n", c.Tracker)
			if len(announceResp.body.Peers) > 0 {
				fmt.Printf("Current peers %v \n", announceResp.body.Peers)
			}
			fmt.Printf("Seeders: %v \n", announceResp.header.Seeders)
			fmt.Printf("Leechers: %v \n", announceResp.header.Leechers)
		}
		return announceResp.body.Peers, nil
	}
	if c.Socket != nil {
		c.Socket.Close()
	}
	return announceResp.body.Peers, errors.New("Failed to request peers")
}

func (m *MagnetURI) newAnnounceRequest(cr connectionResponse) announceRequest {
	ar := announceRequest{
		ConnectionID:  cr.ConnectionID,
		Action:        actionAnnounce,
		TransactionID: newTransactionID(),
		Downloaded:    0,
		Left:          2000000000, //idk how to get this
		Uploaded:      0,
		Event:         eventNone,
		IP:            0,
		Key:           uint32(newTransactionID()),
		NumWant:       -1,
		Port:          port,
	}
	infoHash, _ := hex.DecodeString(m.InfoHash)
	copy(ar.InfoHash[:20], infoHash)
	copy(ar.PeerID[:20], newPeerID())
	return ar
}

func (c *client) connect() (connectionResponse, error) {
	payload := connectionRequest{
		ConnectionID:  connectionID,
		Action:        actionConnect,
		TransactionID: newTransactionID(),
	}
	var response connectionResponse
	raddr, err := net.ResolveUDPAddr("udp", c.Tracker)
	if err != nil {
		return response, err
	}
	c.Socket, err = net.DialUDP("udp", nil, raddr)
	if err != nil {
		return response, err
	}
	_, _, err = c.request(&payload, &response)
	if response.TransactionID != payload.TransactionID {
		return response, errors.New("TransactionID from request does not match response")
	}
	if response.Action != actionConnect {
		return response,
			fmt.Errorf("Connect action response not equal to %d, instead is %d", actionConnect, response.Action)
	}
	return response, nil
}

func (c *client) announce(announceReq announceRequest) (announceResponse, error) {
	var response announceResponse
	readBuffer, bytesRead, err := c.request(&announceReq, &response.header)
	if err != nil {
		return response, errors.New("Failed to announce")
	}
	if bytesRead > announceMinResponseSize {
		numPeers := (bytesRead - announceMinResponseSize) / peerSize
		for i := 0; i < numPeers; i++ {
			var p peer.Peer
			var ipBuf [4]byte
			err = binary.Read(readBuffer, binary.BigEndian, &ipBuf)
			err = binary.Read(readBuffer, binary.BigEndian, &p.Port)
			p.IP = net.IPv4(ipBuf[0], ipBuf[1], ipBuf[2], ipBuf[3])
			if err != nil {
				break
			}
			response.body.Peers = append(response.body.Peers, p)
		}
	}
	return response, nil
}

func (c *client) request(payload interface{}, response interface{}) (*bytes.Buffer, int, error) {
	// timeout: 15 * 2 ^ n (0-8)
	// every 15 * 2 ^ n seconds where n is the number of the request attempt.
	for attempts := 0; attempts < maxRequestAttempts; attempts++ {
		timeoutDuration := time.Second * time.Duration(15*int(math.Pow(2.0, float64(attempts))))
		c.Socket.SetReadDeadline(time.Now().Add(timeoutDuration))

		writeBuffer := bytes.NewBuffer(make([]byte, 0, bufferSize))
		binary.Write(writeBuffer, binary.BigEndian, payload)
		_, err := c.Socket.Write(writeBuffer.Bytes())
		if err != nil {
			return nil, 0, err
		}
		readData := make([]byte, bufferSize)
		bytesRead, err := c.Socket.Read(readData)
		if err, ok := err.(net.Error); ok && err.Timeout() {
			attempts++
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		readBuffer := bytes.NewBuffer(readData[:bytesRead])
		err = binary.Read(readBuffer, binary.BigEndian, response)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, bytesRead, err
		}

		// Success
		return readBuffer, bytesRead, nil
	}
	return nil, 0, errors.New("Failed to connect to make request after 8 attempts")
}
