package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// StateTracker prevents notification spam by only firing on false→true transitions.
type StateTracker struct {
	mu             sync.Mutex
	lastAvailable  bool
	lastNotifyTime time.Time
	cooldown       time.Duration
}

func NewStateTracker() *StateTracker {
	return &StateTracker{
		cooldown: 5 * time.Minute,
	}
}

// ShouldNotify returns true if a notification should be sent.
// Call with available=true when slots found, false otherwise.
func (s *StateTracker) ShouldNotify(available bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !available {
		s.lastAvailable = false
		return false
	}

	// Transition from unavailable → available
	if !s.lastAvailable {
		s.lastAvailable = true
		s.lastNotifyTime = time.Now()
		return true
	}

	// Already available — respect cooldown
	if time.Since(s.lastNotifyTime) >= s.cooldown {
		s.lastNotifyTime = time.Now()
		return true
	}

	return false
}

func SendTelegramMessage(cfg *Config, text string) {
	if cfg.TelegramToken == "" || cfg.TelegramChatID == "" {
		log.Printf("[telegram] skipped (no token/chat_id configured)")
		return
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.TelegramToken)
	body, _ := json.Marshal(map[string]string{
		"chat_id": cfg.TelegramChatID,
		"text":    text,
	})

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[telegram] send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[telegram] unexpected status %d: %s", resp.StatusCode, respBody)
	}
}

func SendTelegramPhoto(cfg *Config, photoPath, caption string) {
	if cfg.TelegramToken == "" || cfg.TelegramChatID == "" {
		log.Printf("[telegram] skipped photo (no token/chat_id configured)")
		return
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", cfg.TelegramToken)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	_ = w.WriteField("chat_id", cfg.TelegramChatID)
	_ = w.WriteField("caption", caption)

	fw, err := w.CreateFormFile("photo", filepath.Base(photoPath))
	if err != nil {
		log.Printf("[telegram] create form file error: %v", err)
		return
	}

	f, err := os.Open(photoPath)
	if err != nil {
		log.Printf("[telegram] open photo error: %v", err)
		return
	}
	defer f.Close()

	if _, err := io.Copy(fw, f); err != nil {
		log.Printf("[telegram] copy photo error: %v", err)
		return
	}
	w.Close()

	resp, err := http.Post(url, w.FormDataContentType(), &buf)
	if err != nil {
		log.Printf("[telegram] send photo error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[telegram] photo unexpected status %d: %s", resp.StatusCode, respBody)
	}
}

func PlayAlertSound() {
	cmd := exec.Command("afplay", "/System/Library/Sounds/Glass.aiff")
	if err := cmd.Run(); err != nil {
		log.Printf("[alert] sound error: %v", err)
	}
}

func SendTelegramNotification(cfg *Config, result *CheckResult) {
	text := fmt.Sprintf("CITA AVAILABLE!\n\n%s", result.Details)
	if result.Screenshot != "" {
		SendTelegramPhoto(cfg, result.Screenshot, text)
	} else {
		SendTelegramMessage(cfg, text)
	}
}
