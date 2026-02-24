# ExampleContainers

This repo now contains a minimal Golang payload service example for Mythic.

## my_agent_service

`my_agent_service` is a minimal payload-type-only service:

- `my_agent/agentfunctions/builder.go` defines payload metadata and build logic
- `my_agent/agentfunctions/echo.go` provides one minimal command (`echo`)
- `my_agent/agent_code/main.go` is a starter implant stub used by the builder

If you have go installed locally, you can test and run via:

- `cd ExampleContainers/Payload_Type/my_agent_service`
- `go mod tidy`
- `go build -o mythic_go_services .`
- `make run_custom` (update the top of the `Makefile` with environment variables you need to set)

### What's happening
At a high level, `main.go` only initializes payload definitions.
In this function call:
```go
MythicContainer.StartAndRunForever([]MythicContainer.MythicServices{
		MythicContainer.MythicServicePayload,
	})
```

The payload service connects to Mythic via RabbitMQ.
Because of this, if you want to run your services remotely (i.e. not within Docker-compose like the Mythic server), then you need to adjust two flags for Mythic's .env:
```text
MYTHIC_SERVER_BIND_LOCALHOST_ONLY="false"
RABBITMQ_BIND_LOCALHOST_ONLY="false"
```
Then restart Mythic, `sudo ./mythic-cli start` so that Docker will bind those ports to `0.0.0.0` instead of `127.0.0.1`.
