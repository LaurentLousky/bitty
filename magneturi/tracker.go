package magneturi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"time"
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
	InfoHash      [20]int8
	PeerID        [20]int8
	Downloaded    int64
	Left          int64
	Uploaded      int64
	Event         int32
	IP            uint32
	Key           uint32
	NumWant       int32
	Port          uint16
	Extensions    uint16
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
	Peers []peer
}

type peer struct {
	IP   int32
	Port uint16
}

type client struct {
	Socket  net.Conn
	Tracker string
}

func newTransactionID() int32 {
	return int32(rand.Uint32())
}

// TODO: Try each tracker in parallel with goroutines
// https://www.bittorrent.org/beps/bep_0012.html
func (m *MagnetURI) requestPeers() error {
	var c client
	var p []peer
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
		p = append(p, announceResp.body.Peers...)
		if announceResp.header.Action == actionAnnounce {
			fmt.Printf("Announced successfully to: %s \n", c.Tracker)
			if len(p) > 0 {
				fmt.Printf("Current peers %v \n", p)
			}
		}
	}
	if c.Socket != nil {
		c.Socket.Close()
	}
	if len(p) == 0 {
		return errors.New("Failed to request peers")
	}
	return nil
}

func (m *MagnetURI) newAnnounceRequest(cr connectionResponse) announceRequest {
	ar := announceRequest{
		ConnectionID:  cr.ConnectionID,
		Action:        actionAnnounce,
		TransactionID: newTransactionID(),
		InfoHash:      strToInt8(m.InfoHash),
		PeerID:        strToInt8(newPeerID()),
		Downloaded:    0,
		Left:          0,
		Uploaded:      0,
		Event:         eventNone,
		IP:            0,
		Key:           uint32(newTransactionID()),
		NumWant:       -1,
		Port:          port,
	}
	return ar
}

func strToInt8(str string) [20]int8 {
	length := 20
	bytes := []byte(str[:length])
	var result [20]int8
	for i := 0; i < length; i++ {
		result[i] = int8(bytes[i])
	}
	return result
}

func (c *client) connect() (connectionResponse, error) {
	payload := connectionRequest{
		ConnectionID:  connectionID,
		Action:        0,
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

	// timeout: 15 * 2 ^ n (0-8)
	// every 15 * 2 ^ n seconds where n is the number of the request attempt.
	for attempts := 0; attempts < maxRequestAttempts; attempts++ {
		timeoutDuration := time.Second * time.Duration(15*int(math.Pow(2.0, float64(attempts))))
		c.Socket.SetReadDeadline(time.Now().Add(timeoutDuration))

		writeBuffer := bytes.NewBuffer(make([]byte, 0, bufferSize))
		binary.Write(writeBuffer, binary.BigEndian, &payload)
		_, err = c.Socket.Write(writeBuffer.Bytes())
		if err != nil {
			return response, err
		}
		readData := make([]byte, bufferSize)
		bytesRead, err := c.Socket.Read(readData)
		if err, ok := err.(net.Error); ok && err.Timeout() {
			attempts++
			continue
		}
		if err != nil {
			return response, err
		}

		if err != nil {
			return response, err
		}
		readBuffer := bytes.NewBuffer(readData[:bytesRead])
		err = binary.Read(readBuffer, binary.BigEndian, &response)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return response, err
		}
		if response.TransactionID != payload.TransactionID {
			return response, errors.New("TransactionID from request does not match response")
		}
		if response.Action != actionConnect {
			return response,
				fmt.Errorf("Connect action response not equal to %d, instead is %d", actionConnect, response.Action)
		}
		return response, nil
	}
	return response, errors.New("Failed to connect to tracker after 8 attempts")
}

func (c *client) announce(announceReq announceRequest) (announceResponse, error) {
	var response announceResponse
	// timeout: 15 * 2 ^ n (0-8)
	// every 15 * 2 ^ n seconds where n is the number of the request attempt.
	for attempts := 0; attempts < maxRequestAttempts; attempts++ {
		timeoutDuration := time.Second * time.Duration(15*int(math.Pow(2.0, float64(attempts))))
		c.Socket.SetReadDeadline(time.Now().Add(timeoutDuration))
		writeBuffer := bytes.NewBuffer(make([]byte, 0, bufferSize))
		// TODO: abstract this out to re-usable "read" and "write" methods
		binary.Write(writeBuffer, binary.BigEndian, &announceReq)
		c.Socket.Write(writeBuffer.Bytes())
		readData := make([]byte, bufferSize)
		bytesRead, err := c.Socket.Read(readData)
		if err != nil {
			return response, err
		}
		readBuffer := bytes.NewBuffer(readData[:bytesRead])
		err = binary.Read(readBuffer, binary.BigEndian, &response.header)
		if bytesRead > announceMinResponseSize {
			numPeers := (bytesRead - announceMinResponseSize) / peerSize
			for i := 0; i < numPeers; i++ {
				var p peer
				err = binary.Read(readBuffer, binary.BigEndian, &p)
				if err != nil {
					break
				}
				response.body.Peers = append(response.body.Peers, p)
			}
		}

		return response, nil
	}
	return response, errors.New("Failed to announce after 8 attempts")
}
