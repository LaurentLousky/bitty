package magneturi

import "testing"

func TestParse(t *testing.T) {
	magnetURI := "magnet:?xt=urn:btih:E7F6991C3DC80E62C986521EABCF03AF2420FC9A&dn=Hot%20Rod%20(2007)%20720p%20BrRip%20x264%20-%20YIFY&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2F9.rarbg.to%3A2920%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=udp%3A%2F%2Ftracker.internetwarriors.net%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.pirateparty.gr%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.cyberia.is%3A6969%2Fannounce"
	got := Parse(magnetURI)
	want := MagnetURI{
		InfoHash: "E7F6991C3DC80E62C986521EABCF03AF2420FC9A",
		Name:     "Hot Rod (2007) 720p BrRip x264 - YIFY",
	}
	trackersWanted := map[string]bool{
		"tracker.coppersurfer.tk:6969":       true,
		"9.rarbg.to:2920":                    true,
		"tracker.opentrackr.org:1337":        true,
		"tracker.internetwarriors.net:1337":  true,
		"tracker.leechers-paradise.org:6969": true,
		"tracker.pirateparty.gr:6969":        true,
		"tracker.cyberia.is:6969":            true,
	}

	if got.InfoHash != want.InfoHash {
		t.Errorf("got %s want %s given, %s", got.InfoHash, want.InfoHash, magnetURI)
	}
	if got.Name != want.Name {
		t.Errorf("got %s want %s given, %s", got.Name, want.Name, magnetURI)
	}
	for _, s := range got.Trackers {
		if trackersWanted[s] != true {
			t.Errorf("got tracker url %s not in %v", s, trackersWanted)
		}
	}
}

func TestDownload(t *testing.T) {
	// magnetURI := "magnet:?xt=urn:btih:E7F6991C3DC80E62C986521EABCF03AF2420FC9A&dn=Hot%20Rod%20(2007)%20720p%20BrRip%20x264%20-%20YIFY&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2F9.rarbg.to%3A2920%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=udp%3A%2F%2Ftracker.internetwarriors.net%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.pirateparty.gr%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.cyberia.is%3A6969%2Fannounce"
	magnetURI := "magnet:?xt=urn:btih:AD5E73E1C36BB39AD4DBF66A9DC3401B82FAC916&dn=Grand%20Theft%20Auto%20IV%20The%20Complete%20Edition%20-%20%5BDODI%20Repack%5D&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2F9.rarbg.to%3A2920%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=udp%3A%2F%2Ftracker.internetwarriors.net%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.pirateparty.gr%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.cyberia.is%3A6969%2Fannounce"
	m := Parse(magnetURI)
	got := m.Download()
	if got != nil {
		t.Errorf("Failed to download file")
	}
}
