package main

import (
	payloadtypeFunctions "GoServices/my_agent/agentfunctions"
	"github.com/MythicMeta/MythicContainer"
)

func main() {
	payloadtypeFunctions.Initialize()
	MythicContainer.StartAndRunForever([]MythicContainer.MythicServices{
		MythicContainer.MythicServicePayload,
	})
}
