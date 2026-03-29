package main

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed av_signatures_full.json
var avSignaturesFullJSON []byte

type avSignatureEntry struct {
	Processes []string `json:"processes"`
}

func init() {
	loadFullAVSignatures()
}

// loadFullAVSignatures 在启动时加载完整特征库。
// 如果嵌入的 JSON 解析失败，则自动退回到 collection.go 里的内置精简特征集。
func loadFullAVSignatures() {
	if len(avSignaturesFullJSON) == 0 {
		return
	}

	var raw map[string]avSignatureEntry
	if err := json.Unmarshal(avSignaturesFullJSON, &raw); err != nil {
		return
	}

	loaded := make(map[string][]string, len(raw))
	for product, entry := range raw {
		processes := make([]string, 0, len(entry.Processes))
		seen := make(map[string]struct{})
		for _, process := range entry.Processes {
			trimmed := strings.TrimSpace(process)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			processes = append(processes, trimmed)
		}
		if len(processes) == 0 {
			continue
		}
		loaded[product] = processes
	}

	if len(loaded) == 0 {
		return
	}
	avProcessSignatures = loaded
}
