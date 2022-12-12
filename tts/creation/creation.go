package creation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	tts_server_go "github.com/jing332/tts-server-go"
	"github.com/jing332/tts-server-go/tts"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	tokenUrl  = "https://southeastasia.customvoice.api.speech.microsoft.com/api/texttospeech/v3.0-beta1/accdemopageentry/auth-token"
	voicesUrl = "https://southeastasia.customvoice.api.speech.microsoft.com/api/texttospeech/v3.0-beta1/accdemopage/voices"
	speakUrl  = "https://southeastasia.customvoice.api.speech.microsoft.com/api/texttospeech/v3.0-beta1/accdemopage/speak"
)

var (
	// TokenErr Token已失效
	TokenErr          = errors.New("unauthorized")
	httpStatusCodeErr = errors.New("http状态码不等于200(OK)")
)

type TTS struct {
	Client *http.Client
	token  string
}

func New() *TTS {
	return &TTS{Client: &http.Client{}}
}

// ToSsml 转为完整的SSML
func ToSsml(text string, pro *tts.VoiceProperty) string {
	pro.Api = tts.ApiCreation
	ssml := `<!--ID=B7267351-473F-409D-9765-754A8EBCDE05;Version=1|{\"VoiceNameToIdMapItems\":[{\"Id\":\"` +
		pro.VoiceId + `\",\"Name\":\"Microsoft Server Speech Text to Speech Voice (zh-CN, XiaoxiaoNeural)\",\"ShortName\":\"` +
		pro.VoiceName + `\",\"Locale\":\"zh-CN\",\"VoiceType\":\"StandardVoice\"}]}-->\n<!--ID=5B95B1CC-2C7B-494F-B746-CF22A0E779B7;Version=1|{\"Locales\":{\"zh-CN\":{\"AutoApplyCustomLexiconFiles\":[{}]}}}-->\n` +
		`<speak version=\"1.0\" xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" xml:lang=\"zh-CN\">` +
		strings.ReplaceAll(pro.ElementString(text), `"`, `\"`) + `</speak>`

	return ssml
}

func (t *TTS) GetAudio(text, format string, pro *tts.VoiceProperty) (audio []byte, err error) {
	return t.GetAudioUseContext(nil, text, format, pro)
}

func (t *TTS) GetAudioUseContext(ctx context.Context, text, format string, pro *tts.VoiceProperty) (audio []byte, err error) {
	if t.token == "" {
		s, err := GetToken()
		if err != nil {
			return nil, fmt.Errorf("获取token失败：%v", err)
		}
		t.token = s
	}

	/* 接口限制 文本长度不能超300 */
	if utf8.RuneCountInString(text) > 295 {
		chunks := tts_server_go.ChunkString(text, 290)
		for _, v := range chunks {
			data, err := t.GetAudioUseContext(ctx, v, format, pro)
			if err != nil {
				return nil, err
			}
			audio = append(audio, data...)
		}
		return audio, nil
	}

	ssml := ToSsml(text, pro)
	audio, err = t.speakBySsml(ctx, ssml, format)
	if err != nil {
		if errors.Is(err, TokenErr) { /* Token已失效 */
			t.token = ""
			audio, err = t.GetAudioUseContext(ctx, text, format, pro)
		} else {
			return nil, err
		}
	}

	return audio, nil
}

func (t *TTS) speakBySsml(ctx context.Context, ssml, format string) ([]byte, error) {
	payload := strings.NewReader(`{
    "ssml": "` + ssml + `",
    "ttsAudioFormat": "` + format + `",
    "offsetInPlainText": 0,
    "lengthInPlainText":` + "300" +
		`,"properties": {
        "SpeakTriggerSource": "AccTuningPagePlayButton"
    }
}`)
	req, err := http.NewRequest(http.MethodPost, speakUrl, payload)
	if ctx != nil {
		req = req.WithContext(ctx)
	}

	if err != nil {
		return nil, err
	}
	req.Header.Add("AccDemoPageAuthToken", t.token)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36 Edg/107.0.1418.42")
	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, TokenErr
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK { /* 服务器返回错误 大概率是SSML格式问题 和 频率过高 */
		return nil, errors.New(string(data))
	}

	return data, nil
}

func GetVoices(token string) ([]byte, error) {
	payload := strings.NewReader(`{"queryCondition":{"items":[{"name":"VoiceTypeList","value":"StandardVoice","operatorKind":"Contains"}]}}`)

	req, err := http.NewRequest(http.MethodPost, voicesUrl, payload)

	if err != nil {
		return nil, err
	}
	req.Header.Add("AccDemoPageAuthToken", token)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	req.Header.Add("X-Ms-Useragent", "SpeechStudio/2021.05.001")
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v: %v, %v", httpStatusCodeErr, resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func GetToken() (string, error) {
	resp, err := http.Get(tokenUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%v: %v, %v", httpStatusCodeErr, resp.StatusCode, resp.Status)
	}

	value := make(map[string]string)
	err = json.NewDecoder(resp.Body).Decode(&value)
	if err != nil {
		return "", err
	}
	return value["authToken"], nil
}
