package creation

import (
	"context"
	"encoding/json"
	"errors"
	tts_server_go "github.com/jing332/tts-server-go"
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
	TokenErr = errors.New("unauthorized")
	// httpStatusCodeErr Http状态码不等于200
	httpStatusCodeErr = errors.New("http status code not equal 200")
)

type TTS struct {
	Client *http.Client
	token  string
}

func New() *TTS {
	return &TTS{Client: &http.Client{}}
}

type SpeakArg struct {
	Text, VoiceName, VoiceId, Rate, Style, StyleDegree, Role, Volume, Format string
}

func (t *TTS) GetAudio(arg *SpeakArg) (audio []byte, err error) {
	return t.GetAudioUseContext(nil, arg)
}

func (t *TTS) GetAudioUseContext(ctx context.Context, arg *SpeakArg) (audio []byte, err error) {
	if t.token == "" {
		s, err := GetToken()
		if err != nil {
			return nil, err
		}
		t.token = s
	}

	/* 接口限制 文本长度不能超300 */
	if utf8.RuneCountInString(arg.Text) > 295 {
		chunks := tts_server_go.ChunkString(arg.Text, 290)
		for _, v := range chunks {
			tmpArg := arg
			tmpArg.Text = v
			data, err := t.GetAudio(tmpArg)
			if err != nil {
				return nil, err
			}
			audio = append(audio, data...)
		}
		return audio, nil
	}
	audio, err = speak(t.Client, ctx, t.token, arg)
	if err != nil {
		if errors.Is(err, TokenErr) { /* Token已失效 */
			t.token = ""
			audio, err = t.GetAudioUseContext(ctx, arg)
		} else {
			return nil, err
		}
	}

	return audio, nil
}

func (t *TTS) GetAudioUseContextBySsml(ctx context.Context, ssml, format string) ([]byte, error) {
	if t.token == "" {
		s, err := GetToken()
		if err != nil {
			return nil, err
		}
		t.token = s
	}

	audio, err := speakBySsml(t.Client, ctx, t.token, ssml, format)
	if err != nil {
		if errors.Is(err, TokenErr) { /* Token已失效 */
			t.token = ""
			audio, err = t.GetAudioUseContextBySsml(ctx, ssml, format)
		} else {
			return nil, err
		}
	}
	return audio, nil
}

func speak(client *http.Client, ctx context.Context, token string, arg *SpeakArg) ([]byte, error) {
	ssml := `<!--ID=B7267351-473F-409D-9765-754A8EBCDE05;Version=1|{\"VoiceNameToIdMapItems\":[{\"Id\":\"` +
		arg.VoiceId + `\",\"Name\":\"Microsoft Server Speech Text to Speech Voice (zh-CN, XiaoxiaoNeural)\",\"ShortName\":\"` +
		arg.VoiceName + `\",\"Locale\":\"zh-CN\",\"VoiceType\":\"StandardVoice\"}]}-->\n<!--ID=5B95B1CC-2C7B-494F-B746-CF22A0E779B7;Version=1|{\"Locales\":{\"zh-CN\":{\"AutoApplyCustomLexiconFiles\":[{}]}}}-->\n<speak version=\"1.0\" xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" xml:lang=\"zh-CN\"><voice name=\"` +
		arg.VoiceName + `\"><lang xml:lang=\"zh-CN\"><mstts:express-as style=\"` + arg.Style + `\" styledegree=\"` + arg.StyleDegree + `\" role=\"` +
		arg.Role + `\"><prosody rate=\"` + arg.Rate + `\" volume=\"` + arg.Volume + `\">` + arg.Text + `</prosody></mstts:express-as></lang></voice></speak>`
	return speakBySsml(client, ctx, token, ssml, arg.Format)
}

func speakBySsml(client *http.Client, ctx context.Context, token, ssml, format string) ([]byte, error) {
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
	req.Header.Add("AccDemoPageAuthToken", token)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
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

	if resp.StatusCode != http.StatusOK { /* 服务器返回错误 大概率是SSML格式有问题 */
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
		return nil, httpStatusCodeErr
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
		return "", httpStatusCodeErr
	}

	value := make(map[string]string)
	err = json.NewDecoder(resp.Body).Decode(&value)
	if err != nil {
		return "", err
	}
	return value["authToken"], nil
}
