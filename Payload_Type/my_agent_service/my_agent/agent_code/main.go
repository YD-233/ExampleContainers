package main

import (
	"fmt"
	"time"
)

var BuildPayloadUUID = ""
var BuildCallbackHost = ""
var BuildCallbackPort = ""

func main() {
	fmt.Printf("my_agent 启动: payload_uuid=%s callback=%s:%s\n", BuildPayloadUUID, BuildCallbackHost, BuildCallbackPort)
	for {
		time.Sleep(60 * time.Second)
	}
}
