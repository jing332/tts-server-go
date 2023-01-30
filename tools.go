package tts_server_go

import (
	uuid "github.com/satori/go.uuid"
	"net"
	"regexp"
	"time"
)

func GetUUID() string {
	return uuid.NewV4().String()
}

func GetISOTime() string {
	T := time.Now().String()
	return T[:23][:10] + "T" + T[:23][11:] + "Z"
}

// ChunkString 根据长度分割string
func ChunkString(s string, chunkSize int) []string {
	if len(s) == 0 {
		return nil
	}
	if chunkSize >= len(s) {
		return []string{s}
	}
	chunks := make([]string, 0, (len(s)-1)/chunkSize+1)
	currentLen := 0
	currentStart := 0
	for i := range s {
		if currentLen == chunkSize {
			chunks = append(chunks, s[currentStart:i])
			currentLen = 0
			currentStart = i
		}
		currentLen++
	}
	chunks = append(chunks, s[currentStart:])
	return chunks
}

// GetOutboundIP 获取本机首选出站IP
func GetOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// GetOutboundIPString 获取本机首选出站IP，如错误则返回 'localhost'
func GetOutboundIPString() string {
	netIp, err := GetOutboundIP()
	if err != nil {
		return "localhost"
	}
	return netIp.String()
}

var charRegexp = regexp.MustCompile(`['"<>&/\\]`)
var entityMap = map[string]string{
	`'`: `&apos;`,
	`"`: `&quot;`,
	`<`: `&lt;`,
	`>`: `&gt;`,
	`&`: `&amp;`,
	`/`: ``,
	`\`: ``,
}

// SpecialCharReplace 过滤掉特殊字符
func SpecialCharReplace(s string) string {
	return charRegexp.ReplaceAllStringFunc(s, func(s2 string) string {
		return entityMap[s2]
	})
}
