package agentfunctions

import (
	"bytes"
	"fmt"
	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var payloadDefinition = agentstructs.PayloadType{
	Name:                                   "my_agent",
	FileExtension:                          "bin",
	Author:                                 "@zhujiayi",
	SupportedOS:                            []string{agentstructs.SUPPORTED_OS_LINUX, agentstructs.SUPPORTED_OS_MACOS, agentstructs.SUPPORTED_OS_WINDOWS},
	Wrapper:                                false,
	CanBeWrappedByTheFollowingPayloadTypes: []string{},
	SupportsDynamicLoading:                 false,
	Description:                            "最小可跑的 Go Agent 模板（用于 Mythic 二次开发）",
	SupportedC2Profiles:                    []string{"http"},
	MythicEncryptsData:                     true,
	MessageFormat:                          agentstructs.MessageFormatJSON,
	BuildParameters: []agentstructs.BuildParameter{
		{
			Name:          "architecture",
			Description:   "选择目标架构",
			Required:      false,
			DefaultValue:  "amd64",
			Choices:       []string{"amd64", "arm64"},
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_CHOOSE_ONE,
		},
	},
	BuildSteps: []agentstructs.BuildStep{
		{
			Name:        "Configure",
			Description: "整理构建参数并生成编译命令",
		},
		{
			Name:        "Compile",
			Description: "编译 my_agent/agent_code",
		},
		{
			Name:        "Finalize",
			Description: "读取构建产物并回传给 Mythic",
		},
	},
}

func build(payloadBuildMsg agentstructs.PayloadBuildMessage) agentstructs.PayloadBuildResponse {
	response := agentstructs.PayloadBuildResponse{
		PayloadUUID:        payloadBuildMsg.PayloadUUID,
		Success:            true,
		UpdatedCommandList: &payloadBuildMsg.CommandList,
	}

	if len(payloadBuildMsg.C2Profiles) == 0 {
		response.Success = false
		response.BuildStdErr = "必须至少选择一个 C2 Profile"
		return response
	}

	goos := "linux"
	switch strings.ToLower(payloadBuildMsg.SelectedOS) {
	case "macos":
		goos = "darwin"
	case "windows":
		goos = "windows"
	}

	goarch, err := payloadBuildMsg.BuildParameters.GetStringArg("architecture")
	if err != nil {
		response.Success = false
		response.BuildStdErr = err.Error()
		return response
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	callbackHost := ""
	callbackPort := ""
	if host, err := payloadBuildMsg.C2Profiles[0].GetStringArg("callback_host"); err == nil {
		callbackHost = host
	}
	if port, err := payloadBuildMsg.C2Profiles[0].GetArg("callback_port"); err == nil {
		callbackPort = fmt.Sprintf("%v", port)
	}

	outputName := fmt.Sprintf("%s-%s-%s.bin", payloadBuildMsg.PayloadUUID, goos, goarch)
	outputPath := filepath.Join(os.TempDir(), outputName)
	ldflags := fmt.Sprintf(
		"-s -w -X 'main.BuildPayloadUUID=%s' -X 'main.BuildCallbackHost=%s' -X 'main.BuildCallbackPort=%s'",
		payloadBuildMsg.PayloadUUID,
		callbackHost,
		callbackPort,
	)
	args := []string{
		"build",
		"-trimpath",
		"-ldflags",
		ldflags,
		"-o",
		outputPath,
		"./my_agent/agent_code",
	}

	mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
		PayloadUUID: payloadBuildMsg.PayloadUUID,
		StepName:    "Configure",
		StepSuccess: true,
		StepStdout:  fmt.Sprintf("GOOS=%s GOARCH=%s output=%s", goos, goarch, outputPath),
	})

	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		response.Success = false
		response.BuildMessage = "编译失败"
		response.BuildStdOut = stdout.String()
		response.BuildStdErr = stderr.String() + "\n" + err.Error()
		mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
			PayloadUUID: payloadBuildMsg.PayloadUUID,
			StepName:    "Compile",
			StepSuccess: false,
			StepStdout:  response.BuildStdErr,
		})
		return response
	}

	mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
		PayloadUUID: payloadBuildMsg.PayloadUUID,
		StepName:    "Compile",
		StepSuccess: true,
		StepStdout:  stdout.String(),
	})

	payloadBytes, err := os.ReadFile(outputPath)
	if err != nil {
		response.Success = false
		response.BuildMessage = "构建成功但读取产物失败"
		response.BuildStdOut = stdout.String()
		response.BuildStdErr = err.Error()
		return response
	}

	response.Payload = &payloadBytes
	response.BuildStdOut = stdout.String()
	response.BuildStdErr = stderr.String()
	response.BuildMessage = "构建成功"
	response.Success = true
	mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
		PayloadUUID: payloadBuildMsg.PayloadUUID,
		StepName:    "Finalize",
		StepSuccess: true,
		StepStdout:  fmt.Sprintf("payload bytes: %d", len(payloadBytes)),
	})
	return response
}

func Initialize() {
	agentstructs.AllPayloadData.Get("my_agent").AddPayloadDefinition(payloadDefinition)
	agentstructs.AllPayloadData.Get("my_agent").AddBuildFunction(build)
}
