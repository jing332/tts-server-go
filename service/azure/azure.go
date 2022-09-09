package azure

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

var wssUrl = `wss://eastus.api.speech.microsoft.com/cognitiveservices/websocket/v1?TricType=AzureDemo&Authorization=bearer%20undefined&X-ConnectionId=`
var conn *websocket.Conn = nil

type TNextReaderCallBack func(*websocket.Conn, int, []byte)

var onNextReader TNextReaderCallBack

func SetWssUrl(host string) {
	if host == "" {
		wssUrl = `wss://eastus.api.speech.microsoft.com/cognitiveservices/websocket/v1?TricType=AzureDemo&Authorization=bearer%20undefined&X-ConnectionId=`
	} else {
		wssUrl = "wss://" + host + "/cognitiveservices/websocket/v1?TricType=AzureDemo&Authorization=bearer%20undefined&X-ConnectionId=`"
		log.Infoln("使用自定义接口地址:", wssUrl)
	}
}

func wssConn() (err error) {
	log.Debugln("创建WebSocket连接...")
	dl := websocket.Dialer{
		EnableCompression: true,
		HandshakeTimeout:  time.Second * 15,
	}

	conn, _, err = dl.Dial(wssUrl, tools.GetHeader(
		`Accept-Encoding:gzip
		User-Agent:Mozilla/5.0 (Linux; Android 7.1.2; M2012K11AC Build/N6F26Q; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/81.0.4044.117 Mobile Safari/537.36
		Origin:https://azure.microsoft.com`,
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
				err := (conn.CloseHandler())(-1, err.Error()) //使用conn的CloseHandler回调
				if err != nil {
					return
				}
				return
			}
		}
	}()

	return nil
}

func sendSsmlMsg(ssml string) error {
	log.Debugln("发送SSML:", ssml)
	msg := "Path: ssml\r\nX-RequestId: " + tools.GetUUID() + "\r\nX-Timestamp: " + service.GetISOTime() + "\r\nContent-Type: application/ssml+xml\r\n\r\n" + ssml
	err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		return fmt.Errorf("发送SSML失败: %s", err)
	}
	return nil
}

func sendPrefixInfo(outputFormat string) error {
	uuid := tools.GetUUID()
	m1 := "Path: speech.config\r\nX-RequestId: " + uuid + "\r\nX-Timestamp: " + service.GetISOTime() +
		"\r\nContent-Type: application/json\r\n\r\n{\"context\":{\"system\":{\"name\":\"SpeechSDK\",\"version\":\"1.19.0\",\"build\":\"JavaScript\",\"lang\":\"JavaScript\",\"os\":{\"platform\":\"Browser/Linux x86_64\",\"name\":\"Mozilla/5.0 (X11; Linux x86_64; rv:78.0) Gecko/20100101 Firefox/78.0\",\"version\":\"5.0 (X11)\"}}}}"
	m2 := "Path: synthesis.context\r\nX-RequestId: " + uuid + "\r\nX-Timestamp: " + service.GetISOTime() +
		"\r\nContent-Type: application/json\r\n\r\n{\"synthesis\":{\"audio\":{\"metadataOptions\":{\"sentenceBoundaryEnabled\":false,\"wordBoundaryEnabled\":false},\"outputFormat\":\"" + outputFormat + "\"}}}"
	err := conn.WriteMessage(websocket.TextMessage, []byte(m1))
	if err != nil {
		return fmt.Errorf("发送Prefix1失败: %s", err)
	}
	err = conn.WriteMessage(websocket.TextMessage, []byte(m2))
	if err != nil {
		return fmt.Errorf("发送Prefix2失败: %s", err)
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
