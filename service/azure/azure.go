package azure

import (
	"fmt"
	"github.com/asters1/tools"
	"github.com/gorilla/websocket"
	tts_server_go "github.com/jing332/tts-server-go"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

var wssUrl = `wss://eastus.api.speech.microsoft.com/cognitiveservices/websocket/v1?TricType=AzureDemo&Authorization=bearer%20undefined&X-ConnectionId=`

type TTS struct {
	wssUrl        string
	uuid          string
	conn          *websocket.Conn
	onReadMessage TReadMessage
}

type TReadMessage func(messageType int, p []byte, errMessage error) (finished bool)

func (t *TTS) NewConn() error {
	log.Infoln("创建WebSocket连接(Azure)...")
	dl := websocket.Dialer{
		EnableCompression: true,
		HandshakeTimeout:  time.Second * 15,
	}

	head := tools.GetHeader(
		`Accept-Encoding:gzip, deflate, br
		User-Agent:Mozilla/5.0 (Linux; Android 7.1.2; M2012K11AC Build/N6F26Q; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/81.0.4044.117 Mobile Safari/537.36
		host:eastus.api.speech.microsoft.com
		Origin:https://azure.microsoft.com`)
	var err error
	t.conn, _, err = dl.Dial(wssUrl+t.uuid, head)
	if err != nil {
		return err
	}

	go func() {
		for {
			if t.conn == nil {
				return
			}
			messageType, p, err := t.conn.ReadMessage()
			closed := t.onReadMessage(messageType, p, err)
			if closed {
				t.conn = nil
				return
			}
		}
	}()

	return nil
}

func (t *TTS) CloseConn() {
	if t.conn != nil {
		t.conn.WriteMessage(websocket.CloseMessage, nil)
		t.conn.Close()
		t.conn = nil
	}
}

func (t *TTS) GetAudio(ssml, format string) (audioData []byte, err error) {
	t.uuid = tools.GetUUID()
	if t.conn == nil {
		err := t.NewConn()
		if err != nil {
			return nil, err
		}
	}

	running := true
	defer func() {
		running = false
	}()
	var finished = make(chan bool)
	var failed = make(chan error)
	t.onReadMessage = func(messageType int, p []byte, errMessage error) bool {
		if messageType == -1 && p == nil && errMessage != nil { //已经断开链接
			if running {
				failed <- errMessage
			}
			return true
		}

		if messageType == 2 {
			index := strings.Index(string(p), "Path:audio")
			data := []byte(string(p)[index+12:])
			audioData = append(audioData, data...)
		} else if messageType == 1 && string(p)[len(string(p))-14:len(string(p))-6] == "turn.end" {
			finished <- true
			return false
		}
		return false
	}
	err = t.sendConfigMessage(format)
	if err != nil {
		return nil, err
	}
	err = t.sendSsmlMessage(ssml)
	if err != nil {
		return nil, err
	}

	select {
	case <-finished:
		return audioData, err
	case errMessage := <-failed:
		return nil, errMessage
	}
}

func (t *TTS) sendConfigMessage(format string) error {
	time := tts_server_go.GetISOTime()
	m1 := "Path: speech.config\r\nX-RequestId: " + t.uuid + "\r\nX-Timestamp: " + time +
		"\r\nContent-Type: application/json\r\n\r\n{\"context\":{\"system\":{\"name\":\"SpeechSDK\",\"version\":\"1.19.0\",\"build\":\"JavaScript\",\"lang\":\"JavaScript\",\"os\":{\"platform\":\"Browser/Linux x86_64\",\"name\":\"Mozilla/5.0 (X11; Linux x86_64; rv:78.0) Gecko/20100101 Firefox/78.0\",\"version\":\"5.0 (X11)\"}}}}"
	m2 := "Path: synthesis.context\r\nX-RequestId: " + t.uuid + "\r\nX-Timestamp: " + time +
		"\r\nContent-Type: application/json\r\n\r\n{\"synthesis\":{\"audio\":{\"metadataOptions\":{\"sentenceBoundaryEnabled\":false,\"wordBoundaryEnabled\":false},\"outputFormat\":\"" + format + "\"}}}"
	err := t.conn.WriteMessage(websocket.TextMessage, []byte(m1))
	if err != nil {
		return fmt.Errorf("发送Config1失败: %s", err)
	}
	err = t.conn.WriteMessage(websocket.TextMessage, []byte(m2))
	if err != nil {
		return fmt.Errorf("发送Config2失败: %s", err)
	}

	return nil
}

func (t *TTS) sendSsmlMessage(ssml string) error {
	msg := "Path: ssml\r\nX-RequestId: " + t.uuid + "\r\nX-Timestamp: " + tts_server_go.GetISOTime() + "\r\nContent-Type: application/ssml+xml\r\n\r\n" + ssml
	err := t.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return fmt.Errorf("发送SSML失败: %s", err)
	}
	return nil
}
