package magneturi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
)

const (
	port           uint16 = 6888
	bufferSize            = 2048
	connectionID          = 0x41727101980
	actionConnect         = 0
	actionAnnounce        = 1
	actionError           = 3
	eventNone             = 0
	eventCompleted        = 1
	eventStarted          = 2
	eventStopped          = 3
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
	Action        int32
	TransactionID int32
	Interval      int32
	Leechers      int32
	Seeders       int32
	// Peers         [1024]byte
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
	for _, tracker := range m.Trackers {
		c.Tracker = tracker
		connectResp, err := c.connect()
		if err != nil {
			continue
		}
		announceReq := m.newAnnounceRequest(connectResp)
		announceResp, err := c.announce(announceReq)
		fmt.Println(announceResp)
		if err != nil {
			continue
		}
		return nil
	}
	if c.Socket != nil {
		c.Socket.Close()
	}
	return errors.New("Failed to request peers")
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

func strToInt8(infoHash string) [20]int8 {
	length := 20
	bytes := []byte(infoHash[:length])
	var hash [20]int8
	for i := 0; i < length; i++ {
		hash[i] = int8(bytes[i])
	}
	return hash
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

	// TODO: set timeouts: it should try the request again up to 8 times,
	// attempts := 0
	// maxAttempts := 8
	// var reqWait time.Duration = 15
	// every 15 * 2 ^ n seconds where n is the number of the request attempt.

	writeBuffer := bytes.NewBuffer(make([]byte, 0, bufferSize))
	binary.Write(writeBuffer, binary.BigEndian, &payload)
	_, err = c.Socket.Write(writeBuffer.Bytes())
	if err != nil {
		return response, err
	}
	readData := make([]byte, bufferSize)
	bytesRead, err := c.Socket.Read(readData)

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

func (c *client) announce(announceReq announceRequest) (announceResponse, error) {
	var response announceResponse
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
	err = binary.Read(readBuffer, binary.BigEndian, &response)
	println(bytesRead)
	return response, nil
}
