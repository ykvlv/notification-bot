package assets

import "embed"

//go:embed *.mp3
var AudioFS embed.FS

func List() []string {
	return []string{
		"Motivation.mp3",
		"Do_It.mp3",
		"Gym.mp3",
	}
}
