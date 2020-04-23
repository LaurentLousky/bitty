package peer

import "net"

// Peer is the IP and Port of a host in the swarm
type Peer struct {
	IP   net.IP
	Port uint16
}

// A block is downloaded by the client when the client is interested in a peer,
// and that peer is not choking the client. A block is uploaded by a client when
// the client is not choking a peer, and that peer is interested in the client.

// It is important for the client to keep its peers informed as to whether or not
// it is interested in them. This state information should be kept up-to-date with
// each peer even when the client is choked. This will allow peers to know if the
// client will begin downloading when it is unchoked (and vice-versa).
type peerConnectionState struct {
	AmChoking      bool
	AmInterested   bool
	PeerChoking    bool
	PeerInterested bool
}

type handshake struct {
	PStrLen  byte
	PStr     string
	Reserved [8]byte
	InfoHash [20]byte
	PeerID   [20]byte
}
