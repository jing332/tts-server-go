package service

import "time"

func GetISOTime() string {
	T := time.Now().String()
	return T[:23][:10] + "T" + T[:23][11:] + "Z"
}
