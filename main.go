package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

func main() {
	cfg := MustLoadConfig()

	if err := os.MkdirAll(cfg.ScreenshotDir, 0755); err != nil {
		log.Fatalf("failed to create screenshot dir: %v", err)
	}

	SendTelegramMessage(cfg, "citasi started. Monitoring for appointments...")

	tracker := NewStateTracker()

	var page *rod.Page
	var cleanup func()

	// killChrome forcefully kills any lingering Chrome processes using our profile.
	killChrome := func() {
		_ = exec.Command("pkill", "-f", "citasi-chrome-profile").Run()
		time.Sleep(2 * time.Second)
	}

	// launchNewBrowser creates a fresh browser instance.
	launchNewBrowser := func() {
		if cleanup != nil {
			// Try graceful close, ignore errors if connection is dead
			func() {
				defer func() { recover() }()
				cleanup()
			}()
			cleanup = nil
		}
		// Kill any lingering Chrome to avoid "Opening in existing browser session"
		killChrome()
		_, page, cleanup = LaunchBrowser(cfg)
	}

	launchNewBrowser()
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	// browserAlive checks if the CDP connection to Chrome is still working.
	browserAlive := func() bool {
		err := rod.Try(func() {
			page.MustEval(`() => document.title`)
		})
		return err == nil
	}

	for cycle := 1; cfg.MaxCycles == 0 || cycle <= cfg.MaxCycles; cycle++ {
		log.Printf("--- Cycle %d ---", cycle)

		if !browserAlive() {
			log.Printf("Browser connection lost, restarting...")
			launchNewBrowser()
		}

		result, err := RunSingleCheck(page, cfg)
		if err != nil {
			log.Printf("Cycle %d error: %v", cycle, err)
			takeErrorScreenshot(page, cfg, cycle)

			if strings.Contains(err.Error(), "WAF") {
				// WAF block: back off and restart browser.
				// The long backoff causes CDP connections to go stale,
				// so we restart with a fresh connection (same profile = same cookies).
				log.Printf("WAF block detected, backing off...")
				backoff := time.Duration(900+rand.IntN(300)) * time.Second
				SendTelegramMessage(cfg, fmt.Sprintf("WAF blocked. Backing off for %s.", backoff.Round(time.Second)))
				log.Printf("Next check in %s", backoff)
				time.Sleep(backoff)
				launchNewBrowser()
				continue
			}

			// Other errors: restart browser and use normal interval
			SendTelegramMessage(cfg, "Cycle error: "+err.Error())
			launchNewBrowser()
		} else if result.Available && tracker.ShouldNotify(true) {
			log.Printf("SLOTS AVAILABLE!")
			SendTelegramNotification(cfg, result)
			PlayAlertSound()
		} else {
			tracker.ShouldNotify(false)
			log.Printf("No availability")
			SendTelegramMessage(cfg, "No citas disponibles.")
		}

		// Navigate to about:blank between cycles to clear page state
		// but keep the same browser session (cookies persist)
		_ = rod.Try(func() {
			page.MustNavigate("about:blank")
		})

		jitter := time.Duration(rand.IntN(cfg.CheckJitter+1)) * time.Second
		sleep := time.Duration(cfg.CheckInterval)*time.Second + jitter
		// Enforce minimum 3-minute interval to avoid WAF rate limits
		minInterval := 180 * time.Second
		if sleep < minInterval {
			sleep = minInterval
		}
		log.Printf("Next check in %s", sleep)
		time.Sleep(sleep)
	}

	log.Printf("Completed %d cycles", cfg.MaxCycles)
	SendTelegramMessage(cfg, "citasi completed all cycles.")
}
