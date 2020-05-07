package peer

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/jackpal/bencode-go"
)

const (
	extMsgHandshake uint8  = 0
	extMsgMetadata  int    = 2
	extMetadata     string = "ut_metadata"
)

const (
	msgTypeRequest    = 0
	msgTypeData       = 1
	msgTypeReject     = 2
	metadataPieceSize = 16384
)

type extMessage struct {
	ID      uint8
	Bencode interface{}
}

type extMetadataMessage struct {
	ID       uint8
	Bencode  metadataResponseDict
	Metadata []byte
}

type extHandshakeDict struct {
	M map[string]int `bencode:"m"`
	// metadataSize int            "metadata_size"
}

type metadataRequest struct {
	Type  int `bencode:"msg_type"`
	Piece int `bencode:"piece"`
}

type fileInfo struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

type torrentInfo struct {
	Pieces      string     `bencode:"pieces"`
	PieceLength int        `bencode:"piece length"`
	Length      int        `bencode:"length"`
	Name        string     `bencode:"name"`
	Files       []fileInfo `bencode:"files"`
}

type metadataResponse struct {
	Bencode  metadataResponseDict
	Metadata []byte
}

type metadataResponseDict struct {
	Type      int `bencode:"msg_type"`
	Piece     int `bencode:"piece"`
	TotalSize int `bencode:"total_size"`
}

// The bit selected for the extension protocol
// is bit 20 from the right (counting starts at 0).
func checkExtensions(h handshake) error {
	if h.Reserved[5]&0x10 == 0x10 {
		return nil
	}
	return errors.New("Peer does not support extensions")
}

// TODO: make sure we dont do this if we already have the metadata
func (p *peerConnection) handleExtMessage(m message) error {
	reader := bytes.NewReader(m.Payload)
	var extMessageID uint8
	binary.Read(reader, binary.BigEndian, &extMessageID)
	if extMessageID == extMsgHandshake {
		var bencodeData extHandshakeDict
		err := bencode.Unmarshal(reader, &bencodeData)
		if err != nil {
			return err
		}
		err = p.extHandshake(bencodeData)
		if err != nil {

		}
	}
	if extMessageID == p.ExtMetadata {
		p.handleExtMetadata(m)
	}
	return nil
}

func (p *peerConnection) extHandshake(bencodeData extHandshakeDict) error {
	m := message{
		ID: msgExtended,
	}
	extM := extMessage{
		ID: extMsgHandshake,
		Bencode: extHandshakeDict{
			M: map[string]int{
				"ut_metadata": extMsgMetadata,
			},
		},
	}
	err := p.writeExtMessage(m, extM)
	if err != nil {
		return err
	}

	if val, ok := bencodeData.M[extMetadata]; ok {
		p.ExtMetadata = uint8(val)
		p.extReqMetadata()
	}
	return nil
}

func (p *peerConnection) extReqMetadata() error {
	extP := metadataRequest{
		Type:  0,
		Piece: 0,
	}
	extM := extMessage{
		ID:      p.ExtMetadata,
		Bencode: extP,
	}
	m := message{
		ID: msgExtended,
	}
	err := p.writeExtMessage(m, extM)
	if err != nil {
		return err
	}

	return nil
}

func (p *peerConnection) handleExtMetadata(m message) error {
	reader := bufio.NewReader(bytes.NewReader(m.Payload))
	var extMsg extMetadataMessage
	var err error = nil
	err = binary.Read(reader, binary.BigEndian, &extMsg.ID)
	err = bencode.Unmarshal(reader, &extMsg.Bencode)

	if err != nil {
		return err
	}
	switch extMsg.Bencode.Type {
	case msgTypeRequest:
		break
	case msgTypeData:
		var buf bytes.Buffer
		reader.WriteTo(&buf)
		extMsg.Metadata = buf.Bytes()
		err = p.recvMetadata(extMsg)
		break
	case msgTypeReject:
		return errors.New("Metadata request rejected by peer")
	}
	return err
}

func (p *peerConnection) recvMetadata(m extMetadataMessage) error {
	reader := bytes.NewReader(m.Metadata)
	if m.Bencode.Piece != p.CurrentMetadataPiece {
		return errors.New("Received the incorrect metadata piece")
	}
	if p.MetadataSize == 0 {
		p.MetadataSize = m.Bencode.TotalSize
	}

	lastPiece := reader.Size() < metadataPieceSize ||
		(p.MetadataSize-(metadataPieceSize*p.CurrentMetadataPiece)) == 0
	_, err := reader.WriteTo(p.MetadataBuff)
	if err != nil {
		return err
	}
	var info torrentInfo
	if lastPiece {
		// decode entire metadata now that we have all the pieces
		fmt.Println("last piece yo.")
		hash := sha1.Sum(p.MetadataBuff.Bytes())
		if hash == p.File.InfoHash {
			err = bencode.Unmarshal(p.MetadataBuff, &info)
			p.File.Metadata = &info
		} else {
			return errors.New("Metadata SHA-1 does not match info hash")
		}
	}
	p.CurrentMetadataPiece++
	return nil
}

// <len><id><extId><ext payload>
func (p *peerConnection) writeExtMessage(m message, extM extMessage) error {
	payload := bytes.NewBuffer(make([]byte, 0, bufferSize))
	err := binary.Write(payload, binary.BigEndian, &extM.ID)
	if err != nil {
		return err
	}
	err = bencode.Marshal(payload, extM.Bencode)
	if err != nil {
		return err
	}
	m.Payload = payload.Bytes()
	err = p.writeMessage(m)
	if err != nil {
		return err
	}
	return nil
}
