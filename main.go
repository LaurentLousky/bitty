package main

import (
	"os"

	"github.com/laurentlousky/stream/magneturi"
)

func main() {
	m := magneturi.Parse(os.Args[1])
	err := m.Download()
	if err != nil {
		println(err.Error())
	}
}
