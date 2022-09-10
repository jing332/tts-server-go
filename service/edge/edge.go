package edge

import (
	"fmt"
	"github.com/asters1/tools"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"
	"tts-server-go/service"
)

var wssUrl = `wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken=6A5AA1D4EAFF4E9FB37E23D68491D6F4&ConnectionId=`
var conn *websocket.Conn = nil

type TNextReaderCallBack func(*websocket.Conn, int, []byte)

var onNextReader TNextReaderCallBack

func wssConn() (err error) {
	log.Debugln("创建WebSocket连接...")

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
	//经测试 空闲140s后服务器则会断开WebSocket连接
	conn.SetCloseHandler(func(code int, text string) error {
		log.Warnln("WebSocket连接已关闭:", text)
		if err := conn.Close(); err != nil {
			return err
		}
		conn = nil
		return nil
	})

	//监听 用来判断连接是否关闭
	go func() {
		for {
			msgType, r, err := conn.ReadMessage()
			onNextReader(conn, msgType, r) //将内容转发 方便处理
			if err != nil {                //有错误 代表WebSocket关闭
				if conn != nil {
					closeHandler := conn.CloseHandler()
					if closeHandler != nil {
						_ = closeHandler(-1, err.Error()) //使用conn的CloseHandler回调
					}
				}
				return
			}
		}
	}()

	return nil
}

func sendPrefixInfo(outputFormat string) error {
	log.Debugln("发送配置消息...")
	cfgMsg := "X-Timestamp:" + service.GetISOTime() + "\r\nContent-Type:application/json; charset=utf-8\r\n" + "Path:speech.config\r\n\r\n" +
		`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"true"},"outputFormat":"` + outputFormat + `"}}}}`

	err := conn.WriteMessage(websocket.TextMessage, []byte(cfgMsg))
	if err != nil {
		return fmt.Errorf("发送配置消息失败: %s", err)
	}

	return nil
}

// 发送SSML消息，其中包括要朗读的文本
func sendSsmlMsg(ssml string) error {
	log.Debugln("发送SSML:", ssml)
	msg := "Path: ssml\r\nX-RequestId: " + tools.GetUUID() + "\r\nX-Timestamp: " + service.GetISOTime() + "\r\nContent-Type: application/ssml+xml\r\n\r\n" + ssml
	err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return fmt.Errorf("发送SSML失败: %s", err)
	}
	return nil
}

func GetAudio(ssml, outputForamt string) ([]byte, error) {
	startTime := time.Now()
	if conn == nil { //无现有WebSocket连接
		err := wssConn() //新建WebSocket连接
		if err != nil {
			return nil, err
		}
	}

	err := sendPrefixInfo(outputForamt)
	if err != nil {
		return nil, fmt.Errorf("发送Prefix消息失败: %s", err)
	}
	err = sendSsmlMsg(ssml)
	if err != nil {
		return nil, fmt.Errorf("发送SSML消息失败: %s", err)
	}

	log.Debugln("接收 消息/音频...")
	var AudioData []byte
	wg := &sync.WaitGroup{}
	wg.Add(1)
	//处理服务器返回内容
	onNextReader = func(c *websocket.Conn, msgType int, body []byte) {
		if msgType == -1 && body == nil { //已经断开链接
			log.Debugln("服务器返回内容为空！已断开WSS连接")
			if wg != nil {
				wg.Done()
			}
			return
		}

		if msgType == 2 {
			index := strings.Index(string(body), "Path:audio")
			data := []byte(string(body)[index+12:])
			AudioData = append(AudioData, data...)
		} else if msgType == 1 && string(body)[len(string(body))-14:len(string(body))-6] == "turn.end" {
			log.Infoln("音频接收完成")
			wg.Done()
			return
		}
	}
	wg.Wait()
	wg = nil

	elapsedTime := time.Since(startTime) / time.Millisecond // duration in ms
	log.Infof("耗时: %dms\n", elapsedTime)

	return AudioData, nil
}
