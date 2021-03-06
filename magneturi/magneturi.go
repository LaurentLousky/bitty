package magneturi

import (
	"encoding/hex"
	"fmt"
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
	fmt.Println("Getting peers...")
	peers, err := m.requestPeers()
	file := &peer.File{
		Name:     m.Name,
		InfoHash: m.InfoHash,
		Peers:    peers,
	}
	fmt.Println("Getting metadata...")
	err = file.GetMetadata()
	if err != nil {
		return err
	}
	fmt.Println("Preparing for download...")
	err = file.Metadata.PrepareForDownload()
	if err != nil {
		return err
	}
	fmt.Println("Beginning download...")
	peer.DownloadMovie(file)
	if err != nil {
		return err
	}
	return nil
}
