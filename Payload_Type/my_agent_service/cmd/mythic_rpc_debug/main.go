package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/MythicMeta/MythicContainer/mythicrpc"
	"github.com/MythicMeta/MythicContainer/rabbitmq"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("用法: mythic_rpc_debug <task|file-create> ...")
	}

	rabbitmq.Initialize()

	switch os.Args[1] {
	case "task":
		runCreateTask(os.Args[2:])
	case "file-create":
		runCreateFile(os.Args[2:])
	case "payload-add-command":
		runPayloadAddCommand(os.Args[2:])
	default:
		fatalf("未知命令: %s", os.Args[1])
	}
}

func runCreateTask(args []string) {
	if len(args) != 3 {
		fatalf("用法: mythic_rpc_debug task <agent_callback_id> <command> <params_json>")
	}

	resp, err := mythicrpc.SendMythicRPCTaskCreate(mythicrpc.MythicRPCTaskCreateMessage{
		AgentCallbackID: args[0],
		CommandName:     args[1],
		Params:          args[2],
	})
	if err != nil {
		fatalf("创建任务失败: %v", err)
	}
	printJSON(resp)
}

func runCreateFile(args []string) {
	if len(args) != 3 {
		fatalf("用法: mythic_rpc_debug file-create <agent_callback_id> <filename> <local_path>")
	}

	fileBytes, err := os.ReadFile(args[2])
	if err != nil {
		fatalf("读取本地文件失败: %v", err)
	}

	resp, err := mythicrpc.SendMythicRPCFileCreate(mythicrpc.MythicRPCFileCreateMessage{
		AgentCallbackID:  args[0],
		Filename:         args[1],
		FileContents:     fileBytes,
		DeleteAfterFetch: false,
	})
	if err != nil {
		fatalf("创建 Mythic 文件失败: %v", err)
	}
	printJSON(resp)
}

func runPayloadAddCommand(args []string) {
	if len(args) < 2 {
		fatalf("用法: mythic_rpc_debug payload-add-command <payload_uuid> <command> [command...]")
	}

	resp, err := mythicrpc.SendMythicRPCPayloadAddCommand(mythicrpc.MythicRPCPayloadAddCommandMessage{
		PayloadUUID: args[0],
		Commands:    args[1:],
	})
	if err != nil {
		fatalf("向 payload 添加命令失败: %v", err)
	}
	printJSON(resp)
}

func printJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatalf("序列化输出失败: %v", err)
	}
	fmt.Println(string(data))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
