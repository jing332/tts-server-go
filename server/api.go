package server

import (
	"context"
	"embed"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	tts_server_go "github.com/jing332/tts-server-go"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jing332/tts-server-go/tts/azure"
	"github.com/jing332/tts-server-go/tts/creation"
	"github.com/jing332/tts-server-go/tts/edge"
	log "github.com/sirupsen/logrus"
)

type GracefulServer struct {
	Token      string
	UseDnsEdge bool

	Server       *http.Server
	serveMux     *http.ServeMux
	shutdownLoad chan struct{}

	edgeLock     sync.Mutex
	azureLock    sync.Mutex
	creationLock sync.Mutex
}

//go:embed public/*
var webFiles embed.FS

// HandleFunc 注册
func (s *GracefulServer) HandleFunc() {
	if s.serveMux == nil {
		s.serveMux = &http.ServeMux{}
	}

	webFilesFs, _ := fs.Sub(webFiles, "public")
	s.serveMux.Handle("/", http.FileServer(http.FS(webFilesFs)))
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

	log.Infof("服务已启动, 监听地址为: %s:%d", tts_server_go.GetOutboundIPString(), port)
	err := s.Server.ListenAndServe()
	if err == http.ErrServerClosed { /*说明调用Shutdown关闭*/
		err = nil
	} else if err != nil {
		return err
	}

	<-s.shutdownLoad /*等待,直到服务关闭*/

	return nil
}

// Close 强制关闭，会终止连接
func (s *GracefulServer) Close() {
	if ttsEdge != nil {
		ttsEdge.CloseConn()
		ttsEdge = nil
	}
	if ttsAzure != nil {
		ttsAzure.CloseConn()
		ttsAzure = nil
	}

	_ = s.Server.Close()
	_ = s.Shutdown(time.Second * 5)
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

// 验证Token true表示成功或未设置Token
func (s *GracefulServer) verifyToken(w http.ResponseWriter, r *http.Request) bool {
	if s.Token != "" {
		token := r.Header.Get("Token")
		if s.Token != token {
			log.Warnf("无效的Token: %s, 远程地址: %s", token, r.RemoteAddr)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("无效的Token"))
			return false
		}
	}
	return true
}

var ttsEdge *edge.TTS

// Microsoft Edge 大声朗读接口
func (s *GracefulServer) edgeAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.edgeLock.Lock()
	defer s.edgeLock.Unlock()
	defer r.Body.Close()
	pass := s.verifyToken(w, r)
	if !pass {
		return
	}

	startTime := time.Now()
	body, _ := io.ReadAll(r.Body)
	ssml := string(body)
	format := r.Header.Get("Format")

	log.Infoln("接收到SSML(Edge):", ssml)
	if ttsEdge == nil {
		ttsEdge = &edge.TTS{DnsLookupEnabled: s.UseDnsEdge}
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
		log.Infof("音频下载完成, 大小：%dKB", len(data)/1024)
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
	log.Infof("耗时：%dms\n", time.Since(startTime).Milliseconds())
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
	pass := s.verifyToken(w, r)
	if !pass {
		return
	}

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
		log.Infof("音频下载完成, 大小：%dKB", len(data)/1024)
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

func (s *GracefulServer) creationAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.creationLock.Lock()
	defer s.creationLock.Unlock()
	defer r.Body.Close()
	pass := s.verifyToken(w, r)
	if !pass {
		return
	}

	startTime := time.Now()
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
		ttsCreation = creation.New()
	}

	var succeed = make(chan []byte)
	var failed = make(chan error)
	go func() {
		for i := 0; i < 3; i++ { /* 循环3次, 成功则return */
			data, err := ttsCreation.GetAudioUseContext(r.Context(), reqData.Text, reqData.Format, reqData.VoiceProperty())
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				if i == 2 { //三次请求都失败
					failed <- err
					return
				}
				log.Warnln(err)
				log.Warnf("开始第%d次重试...", i+1)
				time.Sleep(time.Second * 2)
			} else { /* 成功 */
				succeed <- data
				return
			}
		}
	}()

	select { /* 阻塞 等待结果 */
	case data := <-succeed: /* 成功接收到音频 */
		log.Infof("音频下载完成, 大小：%dKB", len(data)/1024)
		err := writeAudioData(w, data, reqData.Format)
		if err != nil {
			log.Warnln(err)
		}
	case reason := <-failed: /* 失败 */
		writeErrorData(w, http.StatusInternalServerError, "获取音频失败(Creation): "+reason.Error())
		ttsCreation = nil
	case <-r.Context().Done(): /* 与阅读APP断开连接  超时15s */
		log.Warnln("客户端(阅读APP)连接 超时关闭/意外断开")
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
	voiceName := params.Get("voiceName")             /* 发音人 */
	voiceId := params.Get("voiceId")                 /* 发音人ID (Creation接口) */
	secondaryLocale := params.Get("secondaryLocale") /* 二级语言 */
	styleName := params.Get("styleName")             /* 风格 */
	styleDegree := params.Get("styleDegree")         /* 风格强度(0.1-2.0) */
	roleName := params.Get("roleName")               /* 角色(身份) */
	voiceFormat := params.Get("voiceFormat")         /* 音频格式 */
	token := params.Get("token")
	concurrentRate := params.Get("concurrentRate") /* 并发率(请求间隔) 毫秒为单位 */

	var jsonStr []byte
	var err error
	if isCreation == "1" {
		creationJson := &CreationJson{VoiceName: voiceName, VoiceId: voiceId, SecondaryLocale: secondaryLocale, Style: styleName,
			StyleDegree: styleDegree, Role: roleName, Format: voiceFormat}
		jsonStr, err = genLegadoCreationJson(apiUrl, name, creationJson, token, concurrentRate)
	} else {
		jsonStr, err = genLegadoJson(apiUrl, name, voiceName, secondaryLocale, styleName, styleDegree, roleName, voiceFormat, token,
			concurrentRate)
	}
	if err != nil {
		writeErrorData(w, http.StatusBadRequest, err.Error())
	} else {
		_, err := w.Write(jsonStr)
		if err != nil {
			log.Error("网络导入时写入失败：", err)
		}
	}
}

/* 发音人数据 */
func (s *GracefulServer) creationVoicesAPIHandler(w http.ResponseWriter, _ *http.Request) {
	token, err := creation.GetToken()
	if err != nil {
		writeErrorData(w, http.StatusInternalServerError, "获取Token失败: "+err.Error())
		return
	}
	data, err := creation.GetVoices(token)
	if err != nil {
		writeErrorData(w, http.StatusInternalServerError, "获取Voices失败: "+err.Error())
		return
	}
	w.Header().Set("cache-control", "public, max-age=3600, s-maxage=3600")
	_, _ = w.Write(data)
}

func (s *GracefulServer) azureVoicesAPIHandler(w http.ResponseWriter, _ *http.Request) {
	data, err := azure.GetVoices()
	if err != nil {
		writeErrorData(w, http.StatusInternalServerError, "获取Voices失败: "+err.Error())
		return
	}

	w.Header().Set("cache-control", "public, max-age=3600, s-maxage=3600")
	_, _ = w.Write(data)
}
