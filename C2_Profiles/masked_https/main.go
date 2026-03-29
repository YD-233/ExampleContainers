package main

import (
	maskedhttps "MaskedHTTPS/c2functions"
	"MaskedHTTPS/common"
	"github.com/MythicMeta/MythicContainer"
	"log"
	"os"
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
	serverDir, err := filepath.Abs("./server")
	if err != nil {
		log.Printf("清理遗留 server 进程时无法解析目录: %v", err)
		return
	}
	pidFile := common.PIDFilePath(serverDir)
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		_ = os.Remove(pidFile)
		return
	}
	selfPID := os.Getpid()
	if pid <= 0 || pid == selfPID {
		_ = os.Remove(pidFile)
		return
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(pidFile)
		return
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("终止遗留 server 进程失败 pid=%d err=%v", pid, err)
	} else {
		log.Printf("已终止遗留 masked_https_server 进程 pid=%d", pid)
	}
	_ = os.Remove(pidFile)
	time.Sleep(500 * time.Millisecond)
}
