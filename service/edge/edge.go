package edge

import (
	"fmt"
	"github.com/asters1/tools"
	"github.com/gorilla/websocket"
	"github.com/jing332/tts-server-go"
	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"
)

var wssUrl = `wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=6A5AA1D4EAFF4E9FB37E23D68491D6F4&ConnectionId=`
var conn *websocket.Conn = nil

type TNextReaderCallBack func(*websocket.Conn, int, []byte, error) (closed bool)

var onNextReader TNextReaderCallBack

func wssConn() (err error) {
	log.Infoln("创建WebSocket连接(Edge)...")

	dl := websocket.Dialer{
		EnableCompression: true,
		HandshakeTimeout:  time.Second * 15,
	}

	conn, _, err = dl.Dial(wssUrl, tools.GetHeader(
		`Accept-Encoding:gzip
		User-Agent:Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.66 Safari/537.36 Edg/103.0.1264.44
		Origin:chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold
		Host:speech.platform.bing.com`,
	))

	if err != nil {
		return fmt.Errorf("创建WebScoket连接失败:%s", err)
	}

	//监听 用来判断连接是否关闭
	go func() {
		for {
			msgType, r, err := conn.ReadMessage()
			if closed := onNextReader(conn, msgType, r, err); closed {
				conn = nil
				return //连接已关闭 退出监听
			}
		}
	}()

	return nil
}

// CloseConn 强制关闭连接
func CloseConn() {
	if conn != nil { //关闭底层net连接
		conn.Close()
		conn = nil
	}
}

// 发送配置消息，其中包括音频格式
func sendPrefixInfo(outputFormat string) error {
	cfgMsg := "X-Timestamp:" + tts_server_go.GetISOTime() + "\r\nContent-Type:application/json; charset=utf-8\r\n" + "Path:speech.config\r\n\r\n" +
		`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"true"},"outputFormat":"` + outputFormat + `"}}}}`

	err := conn.WriteMessage(websocket.TextMessage, []byte(cfgMsg))
	if err != nil {
		return fmt.Errorf("发送配置消息失败: %s", err)
	}

	return nil
}

// 发送SSML消息，其中包括要朗读的文本
func sendSsmlMsg(ssml string) error {
	log.Infoln("发送SSML:", ssml)
	msg := "Path: ssml\r\nX-RequestId: " + tools.GetUUID() + "\r\nX-Timestamp: " + tts_server_go.GetISOTime() + "\r\nContent-Type: application/ssml+xml\r\n\r\n" + ssml
	err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return fmt.Errorf("发送SSML失败: %s", err)
	}
	return nil
}

func GetAudio(ssml, outputFormat string) ([]byte, error) {
	startTime := time.Now()
	if conn == nil { //无现有WebSocket连接
		err := wssConn() //新建WebSocket连接
		if err != nil {
			return nil, err
		}
	}

	err := sendPrefixInfo(outputFormat)
	if err != nil {
		return nil, fmt.Errorf("发送Prefix消息失败: %s", err)
	}

	var AudioData []byte
	wg := &sync.WaitGroup{}
	wg.Add(1)
	//处理服务器返回内容
	onNextReader = func(c *websocket.Conn, msgType int, body []byte, errMsg error) bool {
		if msgType == -1 && body == nil && errMsg != nil { //已经断开链接
			log.Infoln("服务器已关闭WebSocket连接:", errMsg)
			if wg != nil {
				err = errMsg
				wg.Done() //切回主协程 在接收音频、消息错误时调用
			}
			return true //告诉调用者连接已经关闭了
		}

		if msgType == 2 {
			index := strings.Index(string(body), "Path:audio")
			data := []byte(string(body)[index+12:])
			AudioData = append(AudioData, data...)
		} else if msgType == 1 && string(body)[len(string(body))-14:len(string(body))-6] == "turn.end" {
			log.Infoln("音频接收完成")
			wg.Done()
			return false
		}
		return false
	}

	err = sendSsmlMsg(ssml)
	if err != nil {
		return nil, fmt.Errorf("发送SSML消息失败: %s", err)
	}
	log.Infoln("接收 消息/音频...")
	wg.Wait()
	wg = nil

	elapsedTime := time.Since(startTime) / time.Millisecond // duration in ms
	log.Infof("耗时: %dms\n", elapsedTime)
	if err != nil {
		return nil, err //服务器返回的错误信息
	}

	return AudioData, nil
}

func GetAudioForRetry(ssml, outputFormat string, retryCount int) ([]byte, error) {
	body, err := GetAudio(ssml, outputFormat)
	if err != nil {
		for i := 0; i < retryCount; i++ {
			log.Warnf("第%d次重试...⬇⬇⬇", i+1)
			body, err = GetAudio(ssml, outputFormat)
			if err == nil { //无错误
				break
			}
		}
	}

	return body, err
}
