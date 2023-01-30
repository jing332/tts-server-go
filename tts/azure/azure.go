package azure

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	tsg "github.com/jing332/tts-server-go"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	wssUrl    = `wss://eastus.api.speech.microsoft.com/cognitiveservices/websocket/v1?TricType=AzureDemo&Authorization=bearer%20undefined&X-ConnectionId=`
	voicesUrl = `https://eastus.api.speech.microsoft.com/cognitiveservices/voices/list`
)

type TTS struct {
	DialTimeout  time.Duration
	WriteTimeout time.Duration

	dialContextCancel context.CancelFunc

	uuid          string
	conn          *websocket.Conn
	onReadMessage func(messageType int, p []byte, errMessage error) (finished bool)
}

func (t *TTS) NewConn() error {
	log.Infoln("创建WebSocket连接(Azure)...")
	if t.WriteTimeout == 0 {
		t.WriteTimeout = time.Second * 2
	}
	if t.DialTimeout == 0 {
		t.DialTimeout = time.Second * 3
	}

	dl := websocket.Dialer{
		EnableCompression: true,
	}

	header := http.Header{}
	header.Set("Accept-Encoding", "gzip, deflate, br")
	header.Set("Origin", "https://azure.microsoft.com")
	header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 12; M2012K11AC Build/N6F26Q; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/81.0.4044.117 Mobile Safari/537.36")

	var ctx context.Context
	ctx, t.dialContextCancel = context.WithTimeout(context.Background(), t.DialTimeout)
	defer func() {
		t.dialContextCancel()
		t.dialContextCancel = nil
	}()

	var err error
	var resp *http.Response
	t.conn, resp, err = dl.DialContext(ctx, wssUrl+t.uuid, header)
	if err != nil {
		if resp == nil {
			return err
		}
		return fmt.Errorf("%w: %s", err, resp.Status)
	}

	var size = 0
	go func() {
		for {
			if t.conn == nil {
				return
			}
			messageType, p, err := t.conn.ReadMessage()
			size += len(p)
			if size >= 2000000 { //大于2MB主动断开
				t.onReadMessage(-1, nil, &websocket.CloseError{Code: websocket.CloseAbnormalClosure})
				t.conn = nil
				return
			} else {
				closed := t.onReadMessage(messageType, p, err)
				if closed {
					t.conn = nil
					return
				}
			}
		}
	}()

	return nil
}

func (t *TTS) CloseConn() {
	if t.conn != nil {
		if t.dialContextCancel != nil {
			t.dialContextCancel()
		}
		_ = t.conn.Close()
		t.conn = nil
	}
}

func (t *TTS) GetAudio(ssml, format string) (audioData []byte, err error) {
	err = t.GetAudioStream(ssml, format, func(bytes []byte) {
		audioData = append(audioData, bytes...)
	})
	return audioData, err
}

func (t *TTS) GetAudioStream(ssml, format string, read func([]byte)) error {
	t.uuid = tsg.GetUUID()
	if t.conn == nil {
		err := t.NewConn()
		if err != nil {
			return err
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
			read(data)
		} else if messageType == 1 && string(p)[len(string(p))-14:len(string(p))-6] == "turn.end" {
			finished <- true
			return false
		}
		return false
	}
	err := t.sendConfigMessage(format)
	if err != nil {
		return err
	}
	err = t.sendSsmlMessage(ssml)
	if err != nil {
		return err
	}

	select {
	case <-finished:
		return nil
	case errMessage := <-failed:
		return errMessage
	}
}

func (t *TTS) sendConfigMessage(format string) error {
	timestamp := tsg.GetISOTime()
	m1 := "Path: speech.config\r\nX-RequestId: " + t.uuid + "\r\nX-Timestamp: " + timestamp +
		"\r\nContent-Type: application/json\r\n\r\n{\"context\":{\"system\":{\"name\":\"SpeechSDK\",\"version\":\"1.19.0\",\"build\":\"JavaScript\",\"lang\":\"JavaScript\",\"os\":{\"platform\":\"Browser/Linux x86_64\",\"name\":\"Mozilla/5.0 (X11; Linux x86_64; rv:78.0) Gecko/20100101 Firefox/78.0\",\"version\":\"5.0 (X11)\"}}}}"
	m2 := "Path: synthesis.context\r\nX-RequestId: " + t.uuid + "\r\nX-Timestamp: " + timestamp +
		"\r\nContent-Type: application/json\r\n\r\n{\"synthesis\":{\"audio\":{\"metadataOptions\":{\"sentenceBoundaryEnabled\":false,\"wordBoundaryEnabled\":false},\"outputFormat\":\"" + format + "\"}}}"
	_ = t.conn.SetWriteDeadline(time.Now().Add(t.WriteTimeout))
	err := t.conn.WriteMessage(websocket.TextMessage, []byte(m1))
	if err != nil {
		return fmt.Errorf("发送Config1失败: %s", err)
	}
	_ = t.conn.SetWriteDeadline(time.Now().Add(t.WriteTimeout))
	err = t.conn.WriteMessage(websocket.TextMessage, []byte(m2))
	if err != nil {
		return fmt.Errorf("发送Config2失败: %s", err)
	}

	return nil
}

func (t *TTS) sendSsmlMessage(ssml string) error {
	msg := "Path: ssml\r\nX-RequestId: " + t.uuid + "\r\nX-Timestamp: " + tsg.GetISOTime() + "\r\nContent-Type: application/ssml+xml\r\n\r\n" + ssml
	_ = t.conn.SetWriteDeadline(time.Now().Add(t.WriteTimeout))
	err := t.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return fmt.Errorf("发送SSML失败: %s", err)
	}
	return nil
}

func GetVoices() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, voicesUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36 Edg/107.0.1418.26")
	req.Header.Set("X-Ms-Useragent", "SpeechStudio/2021.05.001")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://azure.microsoft.com")
	req.Header.Set("Referer", "https://azure.microsoft.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
