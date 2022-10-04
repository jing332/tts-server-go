//go:build generate
// +build generate

package main

import (
	"encoding/json"
	"github.com/jing332/tts-server-go/service/creation"
	log "github.com/sirupsen/logrus"
	"html/template"
	"os"
	"strings"
)

const itemTmpl = `// Package voices 本文件由gen_voices.go生成
package voices

var IDs = map[string]string{ {{range .}}
"{{.ShortName}}": "{{.ID}}",{{end}}
}`

func main() {
	items, err := downloadInfo()
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("voices.go")
	if err != nil {
		log.Fatal(err)
	}

	err = template.Must(template.New("").Parse(itemTmpl)).Execute(f, items)
	if err != nil {
		log.Fatal(err)
	}

}

type Item struct {
	ID        string `json:"id"`
	ShortName string `json:"shortName"`
	Locale    string `json:"locale"`
}

type ItemList []struct {
	Item
}

func downloadInfo() ([]*Item, error) {
	t, err := creation.GetToken()
	if err != nil {
		return nil, err
	}
	body, err := creation.GetVoices(t)
	if err != nil {
		return nil, err
	}
	var tmpData []*Item
	err = json.Unmarshal(body, &tmpData)
	if err != nil {
		return nil, err
	}

	var data []*Item
	for _, v := range tmpData {
		if strings.Contains(v.Locale, "zh-") {
			data = append(data, v)
		}
	}

	return data, nil
}
