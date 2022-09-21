package edge

import (
	"fmt"
	"github.com/asters1/tools"
	"github.com/gorilla/websocket"
	"github.com/jing332/tts-server-go"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

var wssUrl = `wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=6A5AA1D4EAFF4E9FB37E23D68491D6F4&ConnectionId=`

type TTS struct {
	wssUrl        string
	uuid          string
	conn          *websocket.Conn
	onReadMessage TReadMessage
}

type TReadMessage func(messageType int, p []byte, errMessage error) (finished bool)

func (t *TTS) NewConn() error {
	log.Infoln("创建WebSocket连接(Edge)...")
	dl := websocket.Dialer{
		EnableCompression: true,
		HandshakeTimeout:  time.Second * 15,
	}

	head := tools.GetHeader(
		`Accept-Encoding:gzip, deflate, br
				User-Agent:Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.66 Safari/537.36 Edg/103.0.1264.44
				Origin:chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold
				Host:speech.platform.bing.com`)
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
	cfgMsg := "X-Timestamp:" + tts_server_go.GetISOTime() + "\r\nContent-Type:application/json; charset=utf-8\r\n" + "Path:speech.config\r\n\r\n" +
		`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"true"},"outputFormat":"` + format + `"}}}}`
	err := t.conn.WriteMessage(websocket.TextMessage, []byte(cfgMsg))
	if err != nil {
		return fmt.Errorf("发送Config失败: %s", err)
	}

	return nil
}

func (t *TTS) sendSsmlMessage(ssml string) error {
	msg := "Path: ssml\r\nX-RequestId: " + t.uuid + "\r\nX-Timestamp: " + tts_server_go.GetISOTime() + "\r\nContent-Type: application/ssml+xml\r\n\r\n" + ssml
	err := t.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return err
	}
	return nil
}
