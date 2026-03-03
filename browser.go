package main

import (
	"os"
	"path/filepath"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/stealth"
)

// LaunchBrowser starts a headed Chrome instance with automation signals fully suppressed.
// Combines three layers of anti-detection:
//  1. Strips Chrome automation flags (--enable-automation, etc.)
//  2. Applies stealth scripts (webdriver, plugins, languages, permissions, chrome.runtime)
//  3. Uses a persistent profile for cookie/session continuity
func LaunchBrowser(cfg *Config) (*rod.Browser, *rod.Page, func()) {
	homeDir, _ := os.UserHomeDir()
	userDataDir := filepath.Join(homeDir, ".citasi-chrome-profile")

	l := launcher.New().
		Headless(false).
		UserDataDir(userDataDir).
		// Strip flags that WAFs use to detect automation
		Delete("enable-automation").
		Delete("disable-extensions").
		Delete("disable-component-extensions-with-background-pages").
		Delete("disable-default-apps").
		Delete("disable-client-side-phishing-detection").
		Delete("disable-sync").
		Delete("disable-background-networking").
		Leakless(false)

	// Use system Chrome
	if _, err := os.Stat("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"); err == nil {
		l = l.Bin("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	}

	controlURL := l.MustLaunch()

	browser := rod.New().
		ControlURL(controlURL).
		MustConnect()

	// Use stealth page — applies comprehensive anti-detection overrides via
	// EvalOnNewDocument (webdriver, plugins, languages, permissions, chrome.runtime, etc.)
	page := stealth.MustPage(browser)

	cleanup := func() {
		browser.MustClose()
	}

	return browser, page, cleanup
}
