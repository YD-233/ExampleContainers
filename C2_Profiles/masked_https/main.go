package main

import (
	maskedhttps "MaskedHTTPS/c2functions"
	"github.com/MythicMeta/MythicContainer"
)

func main() {
	maskedhttps.Initialize()
	MythicContainer.StartAndRunForever([]MythicContainer.MythicServices{
		MythicContainer.MythicServiceC2,
	})
}
