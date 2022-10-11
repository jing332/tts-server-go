package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/jing332/tts-server-go/service/azure"
	"github.com/jing332/tts-server-go/service/creation"
	"github.com/jing332/tts-server-go/service/edge"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type GracefulServer struct {
	Server       *http.Server
	serveMux     *http.ServeMux
	shutdownLoad chan struct{}

	edgeLock     sync.Mutex
	azureLock    sync.Mutex
	creationLock sync.Mutex
}

// HandleFunc 注册
func (s *GracefulServer) HandleFunc() {
	if s.serveMux == nil {
		s.serveMux = &http.ServeMux{}
	}

	s.serveMux.HandleFunc("/", s.webAPIHandler)
	s.serveMux.Handle("/api/legado", http.TimeoutHandler(http.HandlerFunc(s.legadoAPIHandler), 15*time.Second, "timeout"))

	s.serveMux.Handle("/api/azure", http.TimeoutHandler(http.HandlerFunc(s.azureAPIHandler), 30*time.Second, "timeout"))
	s.serveMux.Handle("/api/azure/voices", http.TimeoutHandler(http.HandlerFunc(s.azureVoicesAPIHandler), 30*time.Second, "timeout"))

	s.serveMux.Handle("/api/ra", http.TimeoutHandler(http.HandlerFunc(s.edgeAPIHandler), 30*time.Second, "timeout"))

	s.serveMux.Handle("/api/creation", http.TimeoutHandler(http.HandlerFunc(s.creationAPIHandler), 30*time.Second, "timeout"))
	s.serveMux.Handle("/api/creation/voices", http.TimeoutHandler(http.HandlerFunc(s.creationVoicesAPIHandler), 30*time.Second, "timeout"))
}

// ListenAndServe 监听服务
func (s *GracefulServer) ListenAndServe(port int64) error {
	if s.shutdownLoad == nil {
		s.shutdownLoad = make(chan struct{})
	}

	s.Server = &http.Server{
		Addr:           ":" + strconv.FormatInt(port, 10),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        s.serveMux,
	}
	log.Infoln("服务已启动, 监听端口为", s.Server.Addr)
	err := s.Server.ListenAndServe()
	if err == http.ErrServerClosed { /*说明调用Shutdown关闭*/
		err = nil
	} else if err != nil {
		return err
	}

	<-s.shutdownLoad /*等待,直到服务关闭*/

	return nil
}

// Shutdown 关闭监听服务，需等待响应
func (s *GracefulServer) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := s.Server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("shutdown失败: %d", err)
	}

	close(s.shutdownLoad)
	s.shutdownLoad = nil

	return nil
}

//go:embed public/index.html
var indexHtml []byte

//go:embed public/azure.html
var azureHtml []byte

//go:embed public/creation.html
var creationHtml []byte

func (s *GracefulServer) webAPIHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		w.Write(indexHtml)
	case "/azure.html":
		w.Write(azureHtml)
	case "/creation.html":
		w.Write(creationHtml)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

var ttsEdge *edge.TTS

// Microsoft Edge 大声朗读接口
func (s *GracefulServer) edgeAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.edgeLock.Lock()
	defer s.edgeLock.Unlock()
	defer r.Body.Close()

	startTime := time.Now()
	body, _ := io.ReadAll(r.Body)
	ssml := string(body)
	format := r.Header.Get("Format")

	log.Infoln("接收到SSML(Edge):", ssml)
	if ttsEdge == nil {
		ttsEdge = &edge.TTS{}
	}

	var succeed = make(chan []byte)
	var failed = make(chan error)
	go func() {
		for i := 0; i < 3; i++ { /* 循环3次, 成功则return */
			data, err := ttsEdge.GetAudio(ssml, format)
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) { /* 1006异常断开 */
					log.Infoln("异常断开, 自动重连...")
					time.Sleep(1000) /* 等待一秒 */
				} else { /* 正常性错误，如SSML格式错误 */
					failed <- err
					return
				}
			} else { /* 成功 */
				succeed <- data
				return
			}
		}
	}()

	select { /* 阻塞 等待结果 */
	case data := <-succeed: /* 成功接收到音频 */
		log.Infoln("音频下载完成")
		err := writeAudioData(w, data, format)
		if err != nil {
			log.Warnln(err)
		}
	case reason := <-failed: /* 失败 */
		ttsEdge.CloseConn()
		ttsEdge = nil
		writeErrorData(w, http.StatusInternalServerError, "获取音频失败(Edge): "+reason.Error())
	case <-r.Context().Done(): /* 与阅读APP断开连接 超时15s */
		log.Warnln("客户端(阅读APP)连接 超时关闭/意外断开")
		select { /* 3s内如果成功下载, 就保留与微软服务器的连接 */
		case <-succeed:
			log.Debugln("断开后3s内成功下载")
		case <-time.After(time.Second * 3): /* 抛弃WebSocket连接 */
			ttsEdge.CloseConn()
			ttsEdge = nil
		}
	}
	log.Infof("耗时: %dms\n", time.Since(startTime).Milliseconds())
}

type LastAudioCache struct {
	ssml      string
	audioData []byte
}

var ttsAzure *azure.TTS
var audioCache *LastAudioCache

// 微软Azure TTS接口
func (s *GracefulServer) azureAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.azureLock.Lock()
	defer s.azureLock.Unlock()
	defer r.Body.Close()

	startTime := time.Now()
	format := r.Header.Get("Format")
	body, _ := io.ReadAll(r.Body)
	ssml := string(body)
	log.Infoln("接收到SSML(Azure): ", ssml)

	if audioCache != nil {
		if audioCache.ssml == ssml {
			log.Infoln("与上次超时断开时音频SSML一致, 使用缓存...\n")
			err := writeAudioData(w, audioCache.audioData, format)
			if err != nil {
				log.Warnln(err)
			} else {
				audioCache = nil
			}
			return
		} else { /* SSML不一致, 抛弃 */
			audioCache = nil
		}
	}

	if ttsAzure == nil {
		ttsAzure = &azure.TTS{}
	}

	var succeed = make(chan []byte)
	var failed = make(chan error)
	go func() {
		for i := 0; i < 3; i++ { /* 循环3次, 成功则return */
			data, err := ttsAzure.GetAudio(ssml, format)
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) { /* 1006异常断开 */
					log.Infoln("异常断开, 自动重连...")
					time.Sleep(1000) /* 等待一秒 */
				} else { /* 正常性错误，如SSML格式错误 */
					failed <- err
					return
				}
			} else { /* 成功 */
				succeed <- data
				return
			}
		}
	}()

	select { /* 阻塞 等待结果 */
	case data := <-succeed: /* 成功接收到音频 */
		log.Infoln("音频下载完成")
		err := writeAudioData(w, data, format)
		if err != nil {
			log.Warnln(err)
		}
	case reason := <-failed: /* 失败 */
		ttsAzure.CloseConn()
		ttsAzure = nil
		writeErrorData(w, http.StatusInternalServerError, "获取音频失败(Azure): "+reason.Error())
	case <-r.Context().Done(): /* 与阅读APP断开连接  超时15s */
		log.Warnln("客户端(阅读APP)连接 超时关闭/意外断开")
		select { /* 15s内如果成功下载, 就保留与微软服务器的连接 */
		case data := <-succeed:
			log.Infoln("断开后15s内成功下载")
			audioCache = &LastAudioCache{
				ssml:      ssml,
				audioData: data,
			}
		case <-time.After(time.Second * 15): /* 抛弃WebSocket连接 */
			ttsAzure.CloseConn()
			ttsAzure = nil
		}
	}
	log.Infof("耗时: %dms\n", time.Since(startTime).Milliseconds())
}

var ttsCreation *creation.TTS
var creationLastRequestTime time.Time

func (s *GracefulServer) creationAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.creationLock.Lock()
	defer s.creationLock.Unlock()
	defer r.Body.Close()

	/* 限制两次请求间隔时间 */
	interval := time.Now().Unix() - creationLastRequestTime.Unix()
	if interval <= 2 { /* 两次请求间隔小于等于2s */
		time.Sleep(time.Duration(interval) * time.Second)
	}
	creationLastRequestTime = time.Now()

	startTime := creationLastRequestTime
	body, _ := io.ReadAll(r.Body)
	text := string(body)
	log.Infoln("接收到Json(Creation): ", text)
	var reqData CreationJson
	err := json.Unmarshal(body, &reqData)
	if err != nil {
		writeErrorData(w, http.StatusBadRequest, err.Error())
		return
	}

	if ttsCreation == nil {
		ttsCreation = &creation.TTS{}
	}

	arg := creation.SpeakArg(reqData)
	var data []byte
	if reqData.VoiceId == "" { /* 无VoiceId，向下兼容*/
		data, err = ttsCreation.GetAudioNoVoiceId(&arg)
	} else {
		data, err = ttsCreation.GetAudio(&arg)
	}

	if err != nil {
		writeErrorData(w, http.StatusInternalServerError, "获取音频失败(Creation): "+err.Error())
		ttsCreation = nil
	} else {
		err = writeAudioData(w, data, reqData.Format)
		if err != nil {
			log.Warnln(err)
		}
	}

	log.Infof("耗时: %dms\n", time.Since(startTime).Milliseconds())
}

/* 写入音频数据到客户端(阅读APP) */
func writeAudioData(w http.ResponseWriter, data []byte, format string) error {
	w.Header().Set("Content-Type", formatContentType(format))
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=5")
	_, err := w.Write(data)
	return err
}

/* 写入错误信息到客户端 */
func writeErrorData(w http.ResponseWriter, statusCode int, data string) {
	log.Warnln(data)
	w.WriteHeader(statusCode)
	_, err := w.Write([]byte(data))
	if err != nil {
		log.Warnln(err)
	}
}

/* 阅读APP网络导入API */
func (s *GracefulServer) legadoAPIHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	isCreation := params.Get("isCreation")
	apiUrl := params.Get("api")
	name := params.Get("name")
	voiceName := params.Get("voiceName")     /* 发音人 */
	voiceId := params.Get("voiceId")         /* 发音人ID (Creation接口) */
	styleName := params.Get("styleName")     /* 风格 */
	styleDegree := params.Get("styleDegree") /* 风格强度(0.1-2.0) */
	roleName := params.Get("roleName")       /* 角色(身份) */
	voiceFormat := params.Get("voiceFormat") /* 音频格式 */

	var jsonStr []byte
	var err error
	if isCreation == "1" {
		jsonStr, err = genLegadoCreationJson(apiUrl, name, voiceName, voiceId, styleName, styleDegree, roleName, voiceFormat)
	} else {
		jsonStr, err = genLegodoJson(apiUrl, name, voiceName, styleName, styleDegree, roleName, voiceFormat)
	}
	if err != nil {
		writeErrorData(w, http.StatusBadRequest, err.Error())
	} else {
		w.Write(jsonStr)
	}
}

/* 发音人数据 */
func (s *GracefulServer) creationVoicesAPIHandler(w http.ResponseWriter, r *http.Request) {
	token, err := creation.GetToken()
	if err != nil {
		writeErrorData(w, http.StatusInternalServerError, "获取Token失败: "+err.Error())
	}
	data, err := creation.GetVoices(token)
	if err != nil {
		writeErrorData(w, http.StatusInternalServerError, "获取Voices失败: "+err.Error())
	}
	w.Header().Set("cache-control", "public, max-age=3600, s-maxage=3600")
	w.Write(data)
}

func (s *GracefulServer) azureVoicesAPIHandler(w http.ResponseWriter, r *http.Request) {
	data, err := azure.GetVoices()
	if err != nil {
		writeErrorData(w, http.StatusInternalServerError, "获取Voices失败: "+err.Error())
	}

	w.Header().Set("cache-control", "public, max-age=3600, s-maxage=3600")
	w.Write(data)
}
