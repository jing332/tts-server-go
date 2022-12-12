package tts

import (
	"testing"
)

func TestSsml(t *testing.T) {
	pro := VoiceProperty{Api: ApiCreation, VoiceName: "zh-CN-XiaoxiaoNeural", SecondaryLocale: "en-US",
		VoiceId:   "5f55541d-c844-4e04-a7f8-1723ffbea4a9",
		Prosody:   &Prosody{Rate: 0, Pitch: 0, Volume: 0},
		ExpressAs: &ExpressAs{Style: "angry", StyleDegree: 1.5, Role: "body"}}
	ssml := pro.ElementString("测试文本")
	t.Log(ssml)
}

func TestProsody(t *testing.T) {
	p := Prosody{Rate: 0, Volume: 0, Pitch: 0}
	t.Log(p.ElementString("测试文本"))
}
