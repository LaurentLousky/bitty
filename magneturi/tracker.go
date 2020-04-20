package magneturi

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"
)

const (
	port           = 6888
	bufferSize     = 1024
	reqWaitSeconds = 15
	connectionID   = 0x41727101980
	actionConnect  = 0
	actionAnnounce = 1
	actionError    = 3
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
	port          uint16
	Extensions    uint16
}

type announceResponse struct {
	Action        int32
	TransactionID int32
	Interval      int32
	Leechers      int32
	Seeders       int32
	Peers         []peer
}

type peer struct {
	IP   int32
	Port uint16
}

func newTransactionID() int32 {
	return int32(rand.Uint32())
}

func (m *MagnetURI) requestPeers() error {
	for _, tracker := range m.Trackers {
		connectResp, err := m.connect(tracker)
		if err != nil {
			continue
		}
		err = m.announce(connectResp)
		if err != nil {
			continue
		}
		return nil
	}
	return errors.New("Failed to request peers")
}

func (m *MagnetURI) connect(tracker string) (connectionResponse, error) {
	payload := connectionRequest{
		ConnectionID:  connectionID,
		Action:        0,
		TransactionID: newTransactionID(),
	}
	var response connectionResponse

	raddr, err := net.ResolveUDPAddr("udp", tracker)
	if err != nil {
		return response, err
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return response, err
	}
	defer conn.Close()

	writeBuffer := bytes.NewBuffer(make([]byte, 0, bufferSize))

	binary.Write(writeBuffer, binary.BigEndian, payload)
	_, err = conn.Write(writeBuffer.Bytes())
	if err != nil {
		return response, err
	}
	readData := make([]byte, bufferSize)
	conn.SetReadDeadline(time.Now().Add(reqWaitSeconds * time.Second))
	bytesRead, err := conn.Read(readData)
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

func (m *MagnetURI) announce(cr connectionResponse) error {
	// TODO
	return nil
}
