package main

import "embed"

//go:embed web/player.html web/admin.html
var webFS embed.FS

var playerHTML []byte
var adminHTML []byte

func init() {
	data, err := webFS.ReadFile("web/player.html")
	if err != nil {
		panic("failed to embed player.html: " + err.Error())
	}
	playerHTML = data

	data, err = webFS.ReadFile("web/admin.html")
	if err != nil {
		panic("failed to embed admin.html: " + err.Error())
	}
	adminHTML = data
}
