package samplefocus

import (
	"context"
	"encoding/json"
	"fmt"
	"kitbuilder/config"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func NewChromeContext(config *config.Config) (context.Context, context.CancelFunc) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", false),
		chromedp.Flag("exclude-switches", "enable-automation"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	}

	if config.GoogleChromePath != "" {
		opts = append(opts, chromedp.ExecPath(config.GoogleChromePath))
	}

	allocCtx, cancel1 := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel2 := chromedp.NewContext(allocCtx)

	// load cookies if available
	cookieData, err := os.ReadFile("cookies.json")
	if err == nil {
		var cookies []*network.Cookie
		if err := json.Unmarshal(cookieData, &cookies); err == nil {
			chromedp.Run(ctx,
				chromedp.Navigate("https://samplefocus.com"),
				chromedp.ActionFunc(func(ctx context.Context) error {
					for _, cookie := range cookies {
						network.SetCookie(cookie.Name, cookie.Value).
							WithDomain(cookie.Domain).
							WithPath(cookie.Path).
							WithHTTPOnly(cookie.HTTPOnly).
							WithSecure(cookie.Secure).
							Do(ctx)
					}
					return nil
				}),
			)
		}
	}

	// inject stealth JS
	chromedp.ListenTarget(ctx, func(ev any) {
		if _, ok := ev.(*page.EventDomContentEventFired); ok {
			chromedp.Evaluate(stealthJS, nil).Do(ctx)
		}
	})

	// Combined cancel function
	cancelFunc := func() {
		cancel2()
		cancel1()
	}

	return ctx, cancelFunc
}

func detectAndWaitForCaptcha(ctx context.Context) error {
	var title string
	if err := chromedp.Run(ctx, chromedp.Title(&title)); err != nil {
		return err
	}

	// check for Cloudflare captcha indicators
	if strings.Contains(strings.ToLower(title), "just a moment") ||
		strings.Contains(strings.ToLower(title), "attention required") ||
		strings.Contains(strings.ToLower(title), "cloudflare") {
		log.Println("Cloudflare CAPTCHA detected - waiting for completion...")

		// wait for captcha to be solved (either automatically or manually)
		// check every second for up to 60 seconds
		for i := 0; i < 60; i++ {
			time.Sleep(1 * time.Second)

			var currentTitle string
			err := chromedp.Run(ctx, chromedp.Title(&currentTitle))
			if err != nil {
				// if we get an error, the page might be navigating
				continue
			}

			// check if captcha is gone
			if !strings.Contains(strings.ToLower(currentTitle), "just a moment") &&
				!strings.Contains(strings.ToLower(currentTitle), "attention required") &&
				!strings.Contains(strings.ToLower(currentTitle), "cloudflare") {
				log.Println("✓ CAPTCHA completed!")
				// give it an extra second to fully load
				time.Sleep(2 * time.Second)
				return nil
			}

			if i%5 == 0 && i > 0 {
				log.Printf("Still waiting for CAPTCHA... (%d seconds)", i)
			}
		}

		return fmt.Errorf("timed out waiting for CAPTCHA to complete")
	}

	return nil
}

func FetchSamples(ctx context.Context, search string) ([]Sample, error) {
	url := fmt.Sprintf("https://samplefocus.com/samples?search=%s", search)

	var rawJSON string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(1*time.Second), // Give page time to load
	)
	if err != nil {
		return nil, fmt.Errorf("navigation failed: %w", err)
	}

	if err := detectAndWaitForCaptcha(ctx); err != nil {
		return nil, fmt.Errorf("captcha handling failed: %w", err)
	}

	err = chromedp.Run(ctx,
		// wait for React JSON to appear
		chromedp.Poll(`window.__REACT_ON_RAILS_EVENT_HANDLERS_RAN_ONCE__ === true`, nil,
			chromedp.WithPollingInterval(500*time.Millisecond),
			chromedp.WithPollingTimeout(10*time.Second)),
		// extract script inner text
		chromedp.InnerHTML(`script.js-react-on-rails-component:nth-child(3)`, &rawJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("chromedp failed: %w", err)
	}

	var data reactData
	if err := json.Unmarshal([]byte(rawJSON), &data); err != nil {
		return nil, fmt.Errorf("failed to decode data: %w", err)
	}

	return data.Samples, nil
}
