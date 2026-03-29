package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExecuteSysinfo(t *testing.T) {
	response := executeSysinfo(Task{ID: "task-sysinfo"})
	if response.Status != "success" {
		t.Fatalf("期望 sysinfo 成功，实际状态: %s", response.Status)
	}
	if !response.Completed {
		t.Fatalf("期望 sysinfo 标记完成")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(response.UserOutput), &payload); err != nil {
		t.Fatalf("sysinfo 输出不是有效 JSON: %v", err)
	}
	for _, key := range []string{"hostname", "user", "os", "arch", "pid", "process_name"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("sysinfo 缺少字段: %s", key)
		}
	}
}

func TestDetectSecurityProducts(t *testing.T) {
	processes := []ProcessInfo{
		{Name: "MsSense.exe", PID: 100},
		{Name: "SentinelAgent.exe", PID: 200},
		{Name: "bash", PID: 300},
	}
	detections := detectSecurityProducts(processes)
	if len(detections) < 2 {
		t.Fatalf("期望至少识别两个安全产品，实际: %d", len(detections))
	}

	joined := make([]string, 0, len(detections))
	for _, detection := range detections {
		joined = append(joined, detection.Product)
	}
	names := strings.Join(joined, ",")
	if !strings.Contains(names, "Windows Defender Advanced Threat Protection") {
		t.Fatalf("未识别到 Windows Defender ATP: %s", names)
	}
	if !strings.Contains(names, "SentinelOne") {
		t.Fatalf("未识别到 SentinelOne: %s", names)
	}
}

func TestListProcesses(t *testing.T) {
	processes, err := listProcesses()
	if err != nil {
		t.Fatalf("枚举进程失败: %v", err)
	}
	if len(processes) == 0 {
		t.Fatalf("期望至少枚举到一个进程")
	}
}
