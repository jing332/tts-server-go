package creation

import (
	"context"
	"github.com/jing332/tts-server-go/tts"
	"net/http"
	"testing"
	"time"
)

func TestGetToSsml(t *testing.T) {
	pro := &tts.VoiceProperty{Api: tts.ApiCreation, VoiceName: "zh-CN-XiaoxiaoNeural",
		VoiceId:   "5f55541d-c844-4e04-a7f8-1723ffbea4a9",
		Prosody:   &tts.Prosody{Rate: 0, Pitch: 0, Volume: 0},
		ExpressAs: &tts.ExpressAs{Style: "angry", StyleDegree: 1.5, Role: "body"}}

	t.Log(ToSsml("测试文本", pro))
}

func TestGetAudioUseContext(t *testing.T) {
	pro := &tts.VoiceProperty{Api: tts.ApiCreation, VoiceName: "zh-CN-XiaoxiaoNeural",
		VoiceId:   "5f55541d-c844-4e04-a7f8-1723ffbea4a9",
		Prosody:   &tts.Prosody{Rate: 0, Pitch: 0, Volume: 0},
		ExpressAs: &tts.ExpressAs{Style: "angry", StyleDegree: 1.5, Role: "body"}}

	text := "我是测试文本"
	format := "audio-48khz-96kbitrate-mono-mp3"

	ctx, _ := context.WithCancel(context.Background())
	c := &TTS{Client: &http.Client{Timeout: time.Second * 2}}
	go func() {
		//time.Sleep(500)
		//t.Log("canceled")
		//cancel()
	}()

	for i := 0; i < 1000; i++ {
		data, err := c.GetAudioUseContext(ctx, text, format, pro)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(len(data))
		time.Sleep(5 * time.Second)
	}

}

//
//func TestGetAudio(t *testing.T) {
//	//ssml := `<!--ID=B7267351-473F-409D-9765-754A8EBCDE05;Version=1|{\"VoiceNameToIdMapItems\":[{\"Id\":\"5f55541d-c844-4e04-a7f8-1723ffbea4a9\",\"Name\":\"Microsoft Server Speech Text to Speech Voice (zh-CN, XiaoxiaoNeural)\",\"ShortName\":\"zh-CN-XiaoxiaoNeural\",\"Locale\":\"zh-CN\",\"VoiceType\":\"StandardVoice\"}]}-->\n<!--ID=5B95B1CC-2C7B-494F-B746-CF22A0E779B7;Version=1|{\"Locales\":{\"zh-CN\":{\"AutoApplyCustomLexiconFiles\":[{}]}}}-->\n<speak version=\"1.0\" xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" xml:lang=\"zh-CN\"><voice name=\"zh-CN-XiaoxiaoNeural\"><mstts:express-as style=\"\"><prosody rate=\"0%\" contour=\"\"><say-as interpret-as=\"address\">陕西西安</say-as></prosody></mstts:express-as></voice></speak>`
//	arg := &SpeakArg{
//		Text:        "我是测试文本我是测试文本我是测试文本我是测试文本我是测试文本",
//		VoiceName:   "zh-CN-XiaoxiaoNeural",
//		VoiceId:     "5f55541d-c844-4e04-a7f8-1723ffbea4a9",
//		Rate:        "-50%",
//		Style:       "general",
//		StyleDegree: "1.0",
//		Role:        "default",
//		Volume:      "0%",
//		Format:      "audio-48khz-96kbitrate-mono-mp3",
//	}
//	ctx, cancel := context.WithCancel(context.Background())
//	c := &TTS{Client: &http.Client{Timeout: time.Second * 2}}
//	go func() {
//		time.Sleep(8000 * time.Second)
//		t.Log("cancel")
//		cancel()
//	}()
//	data, err := c.GetAudioUseContext(ctx, arg)
//	if err != nil {
//		t.Fatal(err)
//	}
//	t.Log(len(data))
//}
//
//func TestGetAudioBySsml(t *testing.T) {
//	c := &TTS{Client: &http.Client{Timeout: time.Second * 5}}
//	ssml := `<!--ID=B7267351-473F-409D-9765-754A8EBCDE05;Version=1|{\"VoiceNameToIdMapItems\":[{\"Id\":\"5f55541d-c844-4e04-a7f8-1723ffbea4a9\",\"Name\":\"Microsoft Server Speech Text to Speech Voice (zh-CN, XiaoxiaoNeural)\",\"ShortName\":\"zh-CN-XiaoxiaoNeural\",\"Locale\":\"zh-CN\",\"VoiceType\":\"StandardVoice\"}]}-->\n<!--ID=5B95B1CC-2C7B-494F-B746-CF22A0E779B7;Version=1|{\"Locales\":{\"zh-CN\":{\"AutoApplyCustomLexiconFiles\":[{}]}}}-->\n<speak version=\"1.0\" xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" xml:lang=\"zh-CN\"><voice name=\"zh-CN-XiaoxiaoNeural\"><lang xml:lang=\"zh-CN\"><mstts:express-as style=\"general\" styledegree=\"1.0\" role=\"default\"><prosody rate=\"-50%\" volume=\"0%\">我是测试文本我是测试文本我是测试文本我是测试文本我是测试文本</prosody></mstts:express-as></lang></voice></speak>`
//	audio, err := c.GetAudioUseContextBySsml(nil, ssml, "audio-48khz-96kbitrate-mono-mp3")
//	if err != nil {
//		t.Fatal(err)
//	}
//	t.Log(len(audio))
//}

func TestAuthToken(t *testing.T) {
	token, err := GetToken()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(token)
}

func TestVoices(t *testing.T) {
	token, err := GetToken()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(token)

	b, err := GetVoices(token)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
}
