package main

import (
	"github.com/laurentlousky/stream/magneturi"
)

func main() {
	m := magneturi.Parse("magnet:?xt=urn:btih:E7F6991C3DC80E62C986521EABCF03AF2420FC9A&dn=Hot%20Rod%20(2007)%20720p%20BrRip%20x264%20-%20YIFY&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2F9.rarbg.to%3A2920%2Fannounce&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337&tr=udp%3A%2F%2Ftracker.internetwarriors.net%3A1337%2Fannounce&tr=udp%3A%2F%2Ftracker.leechers-paradise.org%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.pirateparty.gr%3A6969%2Fannounce&tr=udp%3A%2F%2Ftracker.cyberia.is%3A6969%2Fannounce")
	err := m.Download()
	if err != nil {
		println(err.Error())
	}
}
