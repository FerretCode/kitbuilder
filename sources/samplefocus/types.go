package samplefocus

type Sample struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	MP3Url string `json:"sample_mp3_url"`
}

type reactData struct {
	Samples []Sample `json:"samples"`
}
