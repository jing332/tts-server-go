package azure

import (
	"fmt"
	"github.com/asters1/tools"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"
	tts_server_go "tts-server-go"
)

var wssUrl = `wss://eastus.api.speech.microsoft.com/cognitiveservices/websocket/v1?TricType=AzureDemo&Authorization=bearer%20undefined&X-ConnectionId=`
var conn *websocket.Conn = nil

type TNextReaderCallBack func(*websocket.Conn, int, []byte, error) (closed bool)

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
	log.Infoln("创建WebSocket连接(Azure)...")
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

	//监听 用来判断连接是否关闭 (空闲140s后服务器则会断开WebSocket连接)
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

// 发送配置消息，其中包括音频格式
func sendPrefixInfo(outputFormat string) error {
	uuid := tools.GetUUID()
	m1 := "Path: speech.config\r\nX-RequestId: " + uuid + "\r\nX-Timestamp: " + tts_server_go.GetISOTime() +
		"\r\nContent-Type: application/json\r\n\r\n{\"context\":{\"system\":{\"name\":\"SpeechSDK\",\"version\":\"1.19.0\",\"build\":\"JavaScript\",\"lang\":\"JavaScript\",\"os\":{\"platform\":\"Browser/Linux x86_64\",\"name\":\"Mozilla/5.0 (X11; Linux x86_64; rv:78.0) Gecko/20100101 Firefox/78.0\",\"version\":\"5.0 (X11)\"}}}}"
	m2 := "Path: synthesis.context\r\nX-RequestId: " + uuid + "\r\nX-Timestamp: " + tts_server_go.GetISOTime() +
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

	var AudioData []byte
	wg := &sync.WaitGroup{}
	wg.Add(1)
	//处理服务器返回内容
	onNextReader = func(c *websocket.Conn, msgType int, body []byte, errMsg error) bool {
		if msgType == -1 && body == nil && errMsg != nil { //已经断开链接
			log.Infoln("服务器已关闭WebSocket连接", errMsg)
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

	if err != nil {
		return nil, err //服务器返回的错误信息
	}

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
