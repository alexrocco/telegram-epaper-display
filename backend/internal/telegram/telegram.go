// Package telegram ingests channel posts via the Telegram Bot API using long
// polling (getUpdates). The bot must be an administrator of the target channel
// to receive channel_post updates.
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexrocco/telegram-epaper-display/backend/internal/store"
)

// Ingester long-polls Telegram and feeds matching channel posts into the store.
type Ingester struct {
	token       string
	channelID   string // "@username" or numeric chat id
	store       *store.Store
	client      *http.Client
	longPollSec int

	mu    sync.RWMutex
	title string // channel title learned from posts
}

// New creates an Ingester for the given bot token and channel.
func New(token, channelID string, st *store.Store) *Ingester {
	const longPollSec = 30
	return &Ingester{
		token:       token,
		channelID:   channelID,
		store:       st,
		longPollSec: longPollSec,
		client: &http.Client{
			// Slightly longer than the long-poll window so the server reply wins.
			Timeout: time.Duration(longPollSec+15) * time.Second,
		},
	}
}

// Title returns the channel title learned from posts, or "" if unknown yet.
func (in *Ingester) Title() string {
	in.mu.RLock()
	defer in.mu.RUnlock()
	return in.title
}

// Run polls until ctx is cancelled. Transient errors are logged and retried.
func (in *Ingester) Run(ctx context.Context) {
	log.Printf("telegram: ingester started for channel %s", in.channelID)
	for {
		select {
		case <-ctx.Done():
			log.Printf("telegram: ingester stopped")
			return
		default:
		}
		if err := in.poll(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("telegram: poll error: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (in *Ingester) poll(ctx context.Context) error {
	q := url.Values{}
	q.Set("offset", strconv.Itoa(in.store.Offset()))
	q.Set("timeout", strconv.Itoa(in.longPollSec))
	q.Set("allowed_updates", `["channel_post","edited_channel_post"]`)

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?%s", in.token, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := in.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("getUpdates HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out updatesResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("getUpdates not ok: %s", out.Description)
	}

	for _, u := range out.Result {
		post := u.ChannelPost
		if post == nil {
			post = u.EditedChannelPost
		}
		if post == nil || !in.matches(post.Chat) {
			if err := in.store.SetOffset(u.UpdateID + 1); err != nil {
				return err
			}
			continue
		}
		in.setTitle(post.Chat.Title)
		m := store.Message{
			ID:     post.MessageID,
			Text:   post.body(),
			Author: post.AuthorSignature,
			Date:   time.Unix(post.Date, 0),
		}
		if err := in.store.Add(m, u.UpdateID+1); err != nil {
			return err
		}
		log.Printf("telegram: stored message %d (%d chars)", m.ID, len(m.Text))
	}
	return nil
}

func (in *Ingester) matches(c chat) bool {
	if strings.HasPrefix(in.channelID, "@") {
		return strings.EqualFold(in.channelID, "@"+c.Username)
	}
	return in.channelID == strconv.FormatInt(c.ID, 10)
}

func (in *Ingester) setTitle(t string) {
	if t == "" {
		return
	}
	in.mu.Lock()
	in.title = t
	in.mu.Unlock()
}

// --- Bot API response types ---

type updatesResponse struct {
	OK          bool     `json:"ok"`
	Description string   `json:"description"`
	Result      []update `json:"result"`
}

type update struct {
	UpdateID          int      `json:"update_id"`
	ChannelPost       *message `json:"channel_post"`
	EditedChannelPost *message `json:"edited_channel_post"`
}

type message struct {
	MessageID       int    `json:"message_id"`
	Date            int64  `json:"date"`
	Text            string `json:"text"`
	Caption         string `json:"caption"`
	AuthorSignature string `json:"author_signature"`
	Chat            chat   `json:"chat"`
}

// body returns the displayable text: the message text, or the media caption.
func (m *message) body() string {
	if m.Text != "" {
		return m.Text
	}
	return m.Caption
}

type chat struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Username string `json:"username"`
	Type     string `json:"type"`
}
