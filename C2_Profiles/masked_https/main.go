package main

import (
	maskedhttps "MaskedHTTPS/c2functions"
	"bufio"
	"github.com/MythicMeta/MythicContainer"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	cleanupStaleServerProcesses()
	maskedhttps.Initialize()
	MythicContainer.StartAndRunForever([]MythicContainer.MythicServices{
		MythicContainer.MythicServiceC2,
	})
}

func cleanupStaleServerProcesses() {
	serverPath, err := filepath.Abs("./server/masked_https_server")
	if err != nil {
		log.Printf("清理遗留 server 进程时无法解析路径: %v", err)
		return
	}
	cmd := exec.Command("ps", "-ax", "-o", "pid=,command=")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("清理遗留 server 进程时无法枚举进程: %v", err)
		return
	}
	selfPID := os.Getpid()
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid == selfPID {
			continue
		}
		command := strings.Join(fields[1:], " ")
		// 这里仅清理遗留的 masked_https_server 孤儿进程，避免 C2 服务重启后端口被旧进程占用。
		if !strings.Contains(command, "masked_https_server") {
			continue
		}
		if !strings.Contains(command, serverPath) && !strings.HasSuffix(command, "masked_https_server") {
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if err := process.Signal(syscall.SIGTERM); err != nil {
			log.Printf("终止遗留 server 进程失败 pid=%d err=%v", pid, err)
			continue
		}
		log.Printf("已终止遗留 masked_https_server 进程 pid=%d command=%s", pid, command)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("读取进程列表失败: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
}
