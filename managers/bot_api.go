package managers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBotAPIURL = "http://localhost:8787"
	BotMinLevel      = 1100
	BotMaxLevel      = 1900
	BotDefaultLevel  = 1100
)

// BotAPIManager is a thin client for the external Maia-style chess engine
// service. The base URL is taken from the BOT_API_URL env var and falls back
// to localhost:8787 for the dev compose setup.
type BotAPIManager struct {
	baseURL string
	client  *http.Client
}

func NewBotAPIManager() *BotAPIManager {
	base := strings.TrimSpace(os.Getenv("BOT_API_URL"))
	if base == "" {
		base = defaultBotAPIURL
	}
	base = strings.TrimRight(base, "/")
	return &BotAPIManager{
		baseURL: base,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (b *BotAPIManager) BaseURL() string {
	if b == nil {
		return ""
	}
	return b.baseURL
}

type bestMoveRequest struct {
	FEN   string `json:"fen"`
	Level int    `json:"level,omitempty"`
}

type bestMoveResponse struct {
	BestMove string `json:"bestMove"`
	Level    int    `json:"level"`
	Error    string `json:"error,omitempty"`
}

// BestMove asks the bot service for its best move from the given FEN at the
// given level. Returns the move in UCI notation (e.g. "e2e4").
func (b *BotAPIManager) BestMove(fen string, level int) (string, error) {
	if b == nil {
		return "", fmt.Errorf("bot api not configured")
	}

	payload, err := json.Marshal(bestMoveRequest{FEN: fen, Level: level})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, b.baseURL+"/best-move", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("bot api %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed bestMoveResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("bot api: bad response: %w", err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("bot api: %s", parsed.Error)
	}
	if strings.TrimSpace(parsed.BestMove) == "" {
		return "", fmt.Errorf("bot api: empty bestMove")
	}
	return parsed.BestMove, nil
}
