package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kitbuilder/config"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/pkg/browser"
)

const (
	freesoundBaseUrl = "https://freesound.org/apiv2"
)

type AuthorizationResponse struct {
	AccessToken string `json:"access_token"`
}

type SearchResponse struct {
	Results []Sound `json:"results"`
}

type Sound struct {
	Id       int     `json:"id"`
	Name     string  `json:"name"`
	Duration float64 `json:"duration"`
}

func AuthFreeSound(config *config.Config) (string, error) {
	r := http.NewServeMux()
	authenticated := make(chan bool)

	accessToken := ""

	r.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")

		params := make(url.Values)
		params.Set("client_id", config.ClientId)
		params.Set("client_secret", config.ClientSecret)
		params.Set("grant_type", "authorization_code")
		params.Set("code", code)

		req, err := http.NewRequest(
			"POST",
			fmt.Sprintf("%s/oauth2/access_token", freesoundBaseUrl),
			bytes.NewReader([]byte(params.Encode())),
		)
		if err != nil {
			log.Fatal("there was an error creating the request")
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal("there was an error executing the request")
		}

		raw, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatal("there was an error reading the response")
		}

		authResponse := AuthorizationResponse{}
		err = json.Unmarshal(raw, &authResponse)
		if err != nil {
			log.Fatal("there was an error decoding the response")
		}

		accessToken = authResponse.AccessToken
		authenticated <- true

		w.Write([]byte("Authentication successful! You can close this window."))
		log.Println("Authentication with FreeSound complete.")
	})

	go http.ListenAndServe(":3000", r)

	browser.OpenURL(fmt.Sprintf(
		"%s/oauth2/authorize?client_id=%s&response_type=code",
		freesoundBaseUrl,
		config.ClientId,
	))

	<-authenticated

	return accessToken, nil
}

func BuildFreeSoundKit(accessToken string, config *config.Config) {
	for _, category := range config.Categories {
		searchQuery := url.QueryEscape(config.SearchPrefix + " " + category.Name)

		searchUrl, err := url.Parse(fmt.Sprintf("%s/search", freesoundBaseUrl))
		if err != nil {
			log.Fatal("there was an error parsing the base URL")
		}

		params := url.Values{}

		params.Set("query", searchQuery)
		params.Set("page_size", "150")
		params.Set("fields", "id,name,duration,tags,username,license")
		params.Set("filter", fmt.Sprintf("duration:[0 TO %f]", config.MaxDuration))

		searchUrl.RawQuery = params.Encode()

		req, err := http.NewRequest("GET", searchUrl.String(), nil)
		if err != nil {
			log.Fatal("there was an error constructing the request")
		}

		log.Printf("Searching: %s\n", req.URL.String())

		req.Header.Set("Authorization", "Bearer "+accessToken)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal("there was an error fetching the sounds")
		}

		raw, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatal("there was an error decoding the response")
		}

		searchResponse := SearchResponse{}
		err = json.Unmarshal(raw, &searchResponse)
		if err != nil {
			log.Fatal("there was an error parsing the response body: " + err.Error())
		}

		log.Printf(
			"Found %d sounds in category '%s' (max duration: %.1fs)\n",
			len(searchResponse.Results),
			category,
			config.MaxDuration,
		)

		categoryPath := fmt.Sprintf("%s/%s", config.OutputDir, category)
		err = os.MkdirAll(categoryPath, 0755)
		if err != nil {
			log.Printf("error creating category directory: %s\n", err.Error())
			continue
		}

		count := min(category.NumberSounds, len(searchResponse.Results))
		if len(searchResponse.Results) == 0 {
			log.Printf("Warning: No sounds found for category '%s'\n", category)
			continue
		}

		p := rand.Perm(len(searchResponse.Results))
		sounds := make([]Sound, count)

		for i := 0; i < count; i++ {
			sounds[i] = searchResponse.Results[p[i]]
		}

		successCount := 0
		for i := range sounds {
			err = downloadFile(
				categoryPath,
				sounds[i].Name,
				fmt.Sprintf("%s/sounds/%d/download", freesoundBaseUrl, sounds[i].Id),
				accessToken,
				config.DownloadTimeout,
			)
			if err != nil {
				log.Printf("Failed to download sound '%s': %s\n", sounds[i].Name, err.Error())
			} else {
				successCount++
			}
		}

		log.Printf("Downloaded %d/%d sounds for category '%s'\n", successCount, count, category)
	}
}

func downloadFile(dirpath, name, url string, accessToken string, timeoutSeconds int) error {
	filename := fmt.Sprintf("%s/%s.wav", dirpath, name)

	timeout, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req = req.WithContext(timeout)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", res.Status)
	}

	_, err = io.Copy(out, res.Body)
	if err != nil {
		return err
	}

	log.Printf("Downloaded: %s (to %s)\n", name, filename)

	return nil
}
