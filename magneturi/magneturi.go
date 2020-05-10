package magneturi

import (
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/laurentlousky/stream/peer"
)

// MagnetURI https://en.wikipedia.org/wiki/Magnet_URI_scheme
type MagnetURI struct {
	InfoHash [20]byte // xt
	Name     string   // dn
	Trackers []string // tr
}

// Parse converts a Magnet URI string into a MagnetURI struct
func Parse(uri string) MagnetURI {
	queryStr := strings.Split(uri, "magnet:?")[1]
	params, err := url.ParseQuery(queryStr)
	if err != nil {
		panic(err)
	}
	// TODO: Check that magnet uri is actually valid and UDP?
	trackers := params["tr"]
	for i := 0; i < len(trackers); i++ {
		trackers[i] = strings.Split(trackers[i], "udp://")[1]
		trackers[i] = strings.Trim(trackers[i], "/announce")
	}
	magnetURI := MagnetURI{
		Name:     params["dn"][0],
		Trackers: trackers,
	}
	infoHash, _ := hex.DecodeString(strings.Split(params["xt"][0], "urn:btih:")[1])
	copy(magnetURI.InfoHash[:20], infoHash)
	return magnetURI
}

// Download a Magnet URI torrent to the file system
func (m *MagnetURI) Download() error {
	peers, err := m.requestPeers()
	file := &peer.File{
		Name:     m.Name,
		InfoHash: m.InfoHash,
		Peers:    peers,
	}
	err = file.GetMetadata()
	err = file.Metadata.PrepareForDownload()
	if err != nil {
		return err
	}
	peer.Download(file)
	if err != nil {
		return err
	}
	return nil
}
