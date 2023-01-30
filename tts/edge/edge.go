package edge

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	tsg "github.com/jing332/tts-server-go"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	wssUrl = `wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=6A5AA1D4EAFF4E9FB37E23D68491D6F4&ConnectionId=`
)

type TTS struct {
	DnsLookupEnabled bool // 使用DNS解析，而不是北京微软云节点。
	DialTimeout      time.Duration
	WriteTimeout     time.Duration

	dialContextCancel context.CancelFunc

	uuid          string
	conn          *websocket.Conn
	onReadMessage TReadMessage
}

type TReadMessage func(messageType int, p []byte, errMessage error) (finished bool)

func (t *TTS) NewConn() error {
	log.Infoln("创建WebSocket连接(Edge)...")
	if t.WriteTimeout == 0 {
		t.WriteTimeout = time.Second * 2
	}
	if t.DialTimeout == 0 {
		t.DialTimeout = time.Second * 3
	}

	dl := websocket.Dialer{
		EnableCompression: true,
	}

	if !t.DnsLookupEnabled {
		dialer := &net.Dialer{}
		dl.NetDial = func(network, addr string) (net.Conn, error) {
			if addr == "speech.platform.bing.com:443" {
				rand.Seed(time.Now().Unix())
				i := rand.Intn(len(ChinaIpList))
				addr = fmt.Sprintf("%s:443", ChinaIpList[i])
			}
			log.Infoln("connect to IP: " + addr)
			return dialer.Dial(network, addr)
		}
	}

	header := http.Header{}
	header.Set("Accept-Encoding", "gzip, deflate, br")
	header.Set("Origin", "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold")
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.66 Safari/537.36 Edg/103.0.1264.44")

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
		_ = t.conn.Close()
		t.conn = nil
	}
}

func (t *TTS) GetAudio(ssml, format string) (audioData []byte, err error) {
	t.uuid = tsg.GetUUID()
	if t.conn == nil {
		err := t.NewConn()
		if err != nil {
			return nil, err
		}
	}

	running := true
	defer func() { running = false }()
	var finished = make(chan bool)
	var failed = make(chan error)
	t.onReadMessage = func(messageType int, p []byte, errMessage error) bool {
		if messageType == -1 && p == nil && errMessage != nil { //已经断开链接
			if running {
				failed <- errMessage
			}
			return true
		}

		if messageType == websocket.BinaryMessage {
			index := strings.Index(string(p), "Path:audio")
			data := []byte(string(p)[index+12:])
			audioData = append(audioData, data...)
		} else if messageType == websocket.TextMessage && string(p)[len(string(p))-14:len(string(p))-6] == "turn.end" {
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
	cfgMsg := "X-Timestamp:" + tsg.GetISOTime() + "\r\nContent-Type:application/json; charset=utf-8\r\n" + "Path:speech.config\r\n\r\n" +
		`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"false"},"outputFormat":"` + format + `"}}}}`
	_ = t.conn.SetWriteDeadline(time.Now().Add(t.WriteTimeout))
	err := t.conn.WriteMessage(websocket.TextMessage, []byte(cfgMsg))
	if err != nil {
		return fmt.Errorf("发送Config失败: %s", err)
	}

	return nil
}

func (t *TTS) sendSsmlMessage(ssml string) error {
	msg := "Path: ssml\r\nX-RequestId: " + t.uuid + "\r\nX-Timestamp: " + tsg.GetISOTime() + "\r\nContent-Type: application/ssml+xml\r\n\r\n" + ssml
	_ = t.conn.SetWriteDeadline(time.Now().Add(t.WriteTimeout))
	err := t.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return err
	}
	return nil
}
