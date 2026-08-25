package main

import "embed"

//go:embed web/player.html
var webFS embed.FS

var playerHTML []byte

func init() {
	data, err := webFS.ReadFile("web/player.html")
	if err != nil {
		panic("failed to embed player.html: " + err.Error())
	}
	playerHTML = data
}
