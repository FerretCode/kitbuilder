package samplefocus

import (
	"context"
	"fmt"
	"kitbuilder/config"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func BuildSampleFocusKit(config *config.Config) error {
	for i, category := range config.Categories {
		log.Printf("Searching SampleFocus for: '%s' (%d/%d)", category.Name, i+1, len(config.Categories))

		ctx, cancel := NewChromeContext(config)

		searchQuery := config.SearchPrefix + " " + category.Name

		samples, err := FetchSamples(ctx, searchQuery)
		if err != nil {
			log.Printf("error fetching samplefocus sounds: %s\n", err.Error())
			cancel()

			time.Sleep(10 * time.Second)
			continue
		}

		if len(samples) == 0 {
			log.Printf("warning: no samples found for category '%s'\n", category.Name)
			continue
		}

		log.Printf("Found %d samples for '%s'\n", len(samples), category.Name)

		categoryPath := filepath.Join(config.OutputDir, category.Name)
		if err := os.MkdirAll(categoryPath, 0755); err != nil {
			log.Printf("Error creating directory: %s\n", err.Error())
			continue
		}

		count := min(category.NumberSounds, len(samples))

		p := rand.Perm(len(samples))
		chosen := make([]Sample, count)
		for i := 0; i < count; i++ {
			chosen[i] = samples[p[i]]
		}

		success := 0
		for _, s := range chosen {
			err := downloadMP3(ctx, categoryPath, s.Name, s.MP3Url)
			if err != nil {
				log.Printf("Failed to download '%s': %s\n", s.Name, err.Error())
			} else {
				success++
			}

		}

		cancel()

		log.Printf("Downloaded %d/%d samples for '%s'\n", success, count, category.Name)

		if i < len(config.Categories)-1 {
			delay := 10 + rand.Intn(10) // 10-20 seconds
			log.Printf("Waiting %d seconds before next category...\n", delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	return nil
}

func downloadMP3(ctx context.Context, dirpath, name, pageURL string) error {
	filename := filepath.Join(dirpath, name+".mp3")
	if err := os.MkdirAll(dirpath, 0755); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		return err
	}

	mp3Ch := make(chan []byte, 1)
	mp3Requests := make(map[network.RequestID]string)

	chromedp.ListenTarget(ctx, func(ev any) {
		switch ev := ev.(type) {
		case *network.EventResponseReceived:
			if strings.Contains(ev.Response.URL, ".mp3") && ev.Response.MimeType == "audio/mpeg" {
				fmt.Printf("MP3 File Intercepted: RequestID=%s URL=%s\n", ev.RequestID, ev.Response.URL)
				mp3Requests[ev.RequestID] = ev.Response.URL
			}

		case *network.EventLoadingFinished:
			if url, ok := mp3Requests[ev.RequestID]; ok {
				fmt.Printf("MP3 Loading Finished: RequestID=%s URL=%s\n", ev.RequestID, url)

				// get the body in a separate goroutine with timeout
				go func(reqID network.RequestID, reqURL string) {
					c := chromedp.FromContext(ctx)
					execCtx := cdp.WithExecutor(ctx, c.Target)

					// create a timeout context for GetResponseBody
					bodyCtx, cancel := context.WithTimeout(execCtx, 5*time.Second)
					defer cancel()

					fmt.Printf("Fetching response body for RequestID=%s...\n", reqID)
					body, err := network.GetResponseBody(reqID).Do(bodyCtx)
					if err != nil {
						log.Printf("ERROR getting MP3 body for %s: %v\n", reqURL, err)
						return
					}

					fmt.Printf("Got MP3 body: %d bytes\n", len(body))

					select {
					case mp3Ch <- body:
						fmt.Println("Sent MP3 body to channel")
					default:
						log.Println("Channel already has data, skipping")
					}
				}(ev.RequestID, url)

				delete(mp3Requests, ev.RequestID)
			}
		}
	})

	if err := chromedp.Run(ctx,
		chromedp.Navigate(pageURL),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return err
	}

	fmt.Println("Waiting for MP3 body from channel...")
	var mp3Bytes []byte
	select {
	case mp3Bytes = <-mp3Ch:
		fmt.Println("Received MP3 body from channel")
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for MP3 body (tracked %d requests)", len(mp3Requests))
	}

	if len(mp3Bytes) < 1000 {
		return fmt.Errorf("downloaded MP3 seems too small (%d bytes)", len(mp3Bytes))
	}

	if err := os.WriteFile(filename, mp3Bytes, 0644); err != nil {
		return err
	}

	log.Printf("Downloaded: %s → %s (%.2f KB)\n", name, filename, float64(len(mp3Bytes))/1024)
	return nil
}
