// Package api serves the rendered frame and status over HTTP.
//
// The Pico W consumes GET /frame.bin (33600-byte 4-gray buffer) using ETag /
// If-None-Match so it only redraws when the content changes. /frame.png and
// /status are for debugging from a browser.
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"sync"
	"time"

	"github.com/alexxrocco/telegram-epaper-display/backend/internal/render"
	"github.com/alexxrocco/telegram-epaper-display/backend/internal/store"
)

// Frame builds and caches the current display frame from the store. It only
// re-renders when the underlying content changes (tracked by a signature),
// so repeated polls are cheap.
type Frame struct {
	store         *store.Store
	titleFn       func() string
	fallbackTitle string
	messages      int

	mu        sync.Mutex
	sig       string
	buf       []byte
	etag      string
	updatedAt time.Time
}

// NewFrame creates a Frame. titleFn supplies the channel title learned at
// runtime (may return ""), with fallbackTitle used when it does.
func NewFrame(st *store.Store, titleFn func() string, fallbackTitle string, messages int) *Frame {
	return &Frame{
		store:         st,
		titleFn:       titleFn,
		fallbackTitle: fallbackTitle,
		messages:      messages,
	}
}

func (f *Frame) buildView() render.View {
	msgs := f.store.Recent(f.messages)
	title := f.titleFn()
	if title == "" {
		title = f.fallbackTitle
	}
	return render.View{
		ChannelTitle: title,
		Messages:     msgs,
		UpdatedAt:    time.Now(),
		Empty:        len(msgs) == 0,
	}
}

// current returns the cached buffer/etag, re-rendering if content changed.
func (f *Frame) current() (buf []byte, etag string, updatedAt time.Time) {
	v := f.buildView()
	sig := signature(v)

	f.mu.Lock()
	defer f.mu.Unlock()
	if sig != f.sig || f.buf == nil {
		img := render.Render(v)
		f.buf = render.Pack(img)
		sum := sha256.Sum256(f.buf)
		f.etag = `"` + hex.EncodeToString(sum[:])[:16] + `"`
		f.sig = sig
		f.updatedAt = v.UpdatedAt
	}
	return f.buf, f.etag, f.updatedAt
}

func signature(v render.View) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00", v.ChannelTitle)
	for _, m := range v.Messages {
		fmt.Fprintf(h, "%d\x1f%d\x1f%s\x1f%s\x00", m.ID, m.Date.Unix(), m.Author, m.Text)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Handler returns the HTTP mux serving all endpoints.
func (f *Frame) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/frame.bin", f.handleBin)
	mux.HandleFunc("/frame.png", f.handlePNG)
	mux.HandleFunc("/status", f.handleStatus)
	mux.HandleFunc("/", f.handleRoot)
	return mux
}

func (f *Frame) handleBin(w http.ResponseWriter, r *http.Request) {
	buf, etag, _ := f.current()
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(buf)))
	_, _ = w.Write(buf)
}

func (f *Frame) handlePNG(w http.ResponseWriter, r *http.Request) {
	img := render.PreviewImage(render.Render(f.buildView()))
	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, img); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (f *Frame) handleStatus(w http.ResponseWriter, r *http.Request) {
	_, etag, updatedAt := f.current()
	msgs := f.store.Recent(f.messages)
	st := map[string]any{
		"channel":       f.titleFn(),
		"message_count": len(msgs),
		"etag":          etag,
		"updated_at":    updatedAt.Format(time.RFC3339),
		"buffer_bytes":  render.BufferSize,
	}
	if len(msgs) > 0 {
		st["last_message_at"] = msgs[0].Date.Format(time.RFC3339)
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(st)
}

func (f *Frame) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>telegram-epaper-display</title>
<h1>telegram-epaper-display</h1>
<p>Live preview (what the e-paper shows):</p>
<p><img src="/frame.png" style="border:1px solid #ccc;image-rendering:pixelated;width:480px"></p>
<ul>
<li><a href="/frame.png">/frame.png</a> — PNG preview</li>
<li><a href="/frame.bin">/frame.bin</a> — raw 4-gray buffer (Pico endpoint)</li>
<li><a href="/status">/status</a> — JSON status</li>
</ul>`)
}
