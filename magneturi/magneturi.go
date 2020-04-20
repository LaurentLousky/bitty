package magneturi

import (
	"math/rand"
	"net/url"
	"strings"
	"time"
)

// MagnetURI https://en.wikipedia.org/wiki/Magnet_URI_scheme
type MagnetURI struct {
	InfoHash string   // xt
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
	}
	magnetURI := MagnetURI{
		InfoHash: params["xt"][0],
		Name:     params["dn"][0],
		Trackers: trackers,
	}
	return magnetURI
}

// Download a Magnet URI torrent to the file system
func (m *MagnetURI) Download() error {
	err := m.requestPeers()
	if err != nil {
		return err
	}
	return nil
}

func generatePeerID() string {
	peerID := ""
	numChars := 20
	baseASCII := 48
	baseNum := 10
	rand.Seed(time.Now().UTC().UnixNano())
	for i := 0; i < numChars; i++ {
		peerID += (string)(rand.Intn(baseNum) + baseASCII)
	}
	return peerID
}
