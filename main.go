package main

import (
	"kitbuilder/config"
	"kitbuilder/sources"
	"kitbuilder/sources/samplefocus"
	"log"
	"os"
)

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	os.RemoveAll(config.OutputDir)
	err = os.MkdirAll(config.OutputDir, 0755)
	if err != nil {
		log.Fatal("error creating sounds directory: " + err.Error())
	}

	switch config.SamplesSource {
	case "freesound":
		accessToken, err := sources.AuthFreeSound(config)
		if err != nil {
			log.Fatal("error authenticating with freesound: " + err.Error())
		}

		sources.BuildFreeSoundKit(accessToken, config)
	case "samplefocus":
		err = samplefocus.BuildSampleFocusKit(config)
		if err != nil {
			log.Fatal("error building samplefocus kit")
		}
	}

	log.Println("Drumkit building complete!")
}
