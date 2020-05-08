package peer

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"net"
	"time"

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
	metadataPieceSize = 16384 //16KiB
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
}

type metadataRequest struct {
	Type  int `bencode:"msg_type"`
	Piece int `bencode:"piece"`
}

type fileInfo struct {
	Length int      `bencode:"length"`
	Path   []string `bencode:"path"`
}

// TorrentInfo represents the info section of a torrent file (the metadata)
type TorrentInfo struct {
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

// GetMetadata gets the file's metadata from a peer and assigns it to the *File
func (file *File) GetMetadata() error {
	for i := 0; i < len(file.Peers); i++ {
		conn, err := net.DialTimeout("tcp", file.Peers[i].String(), 3*time.Second)
		defer conn.Close()
		if err != nil {
			continue
		}
		p := newPeerConnection(file, conn)
		err = p.handshake()
		if err != nil {
			continue
		}
		for {
			message, err := p.readMessage()
			if err != nil {
				break
			}
			// keep reading until we hit an ext message (it's all we care about for metadata)
			if message.ID == msgExtended {
				metadata, err := p.handleExtMessage(message)
				if err != nil {
					break
				}
				if metadata != nil {
					file.Metadata = metadata
					return nil
				}
			}
		}
		conn.Close()
		continue
	}
	return errors.New("Could not get metadata from any of the peers")
}

// The bit selected for the extension protocol
// is bit 20 from the right (counting starts at 0).
func checkExtensions(h handshake) error {
	if h.Reserved[5]&0x10 == 0x10 {
		return nil
	}
	return errors.New("Peer does not support extensions")
}

func (p *peerConnection) handleExtMessage(m message) (*TorrentInfo, error) {
	reader := bytes.NewReader(m.Payload)
	var extMessageID uint8
	binary.Read(reader, binary.BigEndian, &extMessageID)
	if extMessageID == extMsgHandshake {
		var bencodeData extHandshakeDict
		err := bencode.Unmarshal(reader, &bencodeData)
		if err != nil {
			return nil, err
		}
		err = p.extHandshake(bencodeData)
		if err != nil {
			return nil, err
		}
	}
	if extMessageID == p.ExtMetadata {
		metadata, err := p.handleExtMetadata(m)
		if err != nil {
			return nil, err
		}
		if metadata != nil {
			return metadata, nil
		}
	}
	return nil, errors.New("Unexpected extended message")
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

func (p *peerConnection) handleExtMetadata(m message) (*TorrentInfo, error) {
	reader := bufio.NewReader(bytes.NewReader(m.Payload))
	var extMsg extMetadataMessage
	var err error = nil
	err = binary.Read(reader, binary.BigEndian, &extMsg.ID)
	err = bencode.Unmarshal(reader, &extMsg.Bencode)

	if err != nil {
		return nil, err
	}
	switch extMsg.Bencode.Type {
	case msgTypeRequest:
		break
	case msgTypeData:
		var buf bytes.Buffer
		reader.WriteTo(&buf)
		extMsg.Metadata = buf.Bytes()
		metadata, err := p.recvMetadata(extMsg)
		if err != nil {
			return nil, err
		}
		if metadata != nil {
			return metadata, nil
		}
		break
	case msgTypeReject:
		return nil, errors.New("Metadata request rejected by peer")
	}
	return nil, errors.New("Unknown extended message")
}

func (p *peerConnection) recvMetadata(m extMetadataMessage) (*TorrentInfo, error) {
	reader := bytes.NewReader(m.Metadata)
	if m.Bencode.Piece != p.CurrentMetadataPiece {
		return nil, errors.New("Received the incorrect metadata piece")
	}
	if p.MetadataSize == 0 {
		p.MetadataSize = m.Bencode.TotalSize
	}

	lastPiece := reader.Size() < metadataPieceSize ||
		(p.MetadataSize-(metadataPieceSize*p.CurrentMetadataPiece)) == 0
	_, err := reader.WriteTo(p.MetadataBuff)
	if err != nil {
		return nil, err
	}
	var info TorrentInfo
	if lastPiece {
		// decode entire metadata now that we have all the pieces
		hash := sha1.Sum(p.MetadataBuff.Bytes())
		if hash == p.File.InfoHash {
			err = bencode.Unmarshal(p.MetadataBuff, &info)
			return &info, nil
		}
		return nil, errors.New("Metadata SHA-1 does not match info hash")
	}
	p.CurrentMetadataPiece++
	return nil, nil
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
