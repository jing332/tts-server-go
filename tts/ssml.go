package tts

import (
	"strconv"
)

const (
	ApiEdge     = 0
	ApiAzure    = 1
	ApiCreation = 2
)

type VoiceProperty struct {
	Api             int
	VoiceName       string
	VoiceId         string
	SecondaryLocale string
	*Prosody
	*ExpressAs
}

// ElementString 转为Voice元素字符串
func (v *VoiceProperty) ElementString(text string) string {
	var element string
	if v.Api == ApiEdge {
		element = v.Prosody.ElementString(text)
	} else {
		element = v.ExpressAs.ElementString(text, v.Prosody)
	}
	if v.SecondaryLocale == "" {
		return `<voice name="` + v.VoiceName + `">` + element + `</voice>`
	} else { // 二级语言标签
		return `<voice name="` + v.VoiceName + `"><lang xml:lang="` + v.SecondaryLocale + `">` + element + `</lang></voice>`
	}
}

type Prosody struct {
	Rate, Volume, Pitch int8
}

func (p *Prosody) ElementString(text string) string {
	return `<prosody rate="` + strconv.Itoa(int(p.Rate)) +
		`%" volume="` + strconv.Itoa(int(p.Volume)) +
		`%" pitch="` + strconv.Itoa(int(p.Pitch)) +
		`%">` + text + `</prosody>`
}

type ExpressAs struct {
	Style       string
	StyleDegree float32
	Role        string
}

func (e *ExpressAs) ElementString(text string, prosody *Prosody) string {
	if e.Style == "" {
		e.Style = "general"
	}
	if e.Role == "" {
		e.Role = "default"
	}

	return `<mstts:express-as style="` + e.Style +
		`" styledegree="` + strconv.FormatFloat(float64(e.StyleDegree), 'f', 1, 32) +
		`" role="` + e.Role +
		`">"` + prosody.ElementString(text) +
		`</mstts:express-as>`
}
