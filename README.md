# telegram-epaper-display

Show the messages of a **Telegram channel** on a **Waveshare Pico-ePaper-3.7**
(480×280, 4 grayscale levels) driven by a **Raspberry Pi Pico W**.

Two components:

- **`backend/`** — a Go service that runs on your server. It reads the channel
  via the Telegram Bot API, renders a ready-to-display 4-gray bitmap, and serves
  it over HTTP.
- **`firmware/`** — MicroPython for the Pico W. It just fetches the bitmap over
  WiFi and pushes it to the e-paper.

All the layout/font/word-wrap work happens on the server, so the firmware stays
tiny and the display looks good.

```
Telegram channel
   │  Bot API (getUpdates long polling — no public HTTPS needed)
   ▼
Go backend (server) ──HTTP /frame.bin (33600 bytes, ETag)──▶ Pico W + e-paper
```

## Repository layout

```
backend/
  cmd/server/main.go        entrypoint
  internal/config/          env config
  internal/telegram/        Bot API ingestion (getUpdates)
  internal/store/           recent messages + poll offset (JSON persistence)
  internal/render/          text → 480×280 image → 280×480 GS2_HMSB buffer
  internal/api/             HTTP: /frame.bin, /frame.png, /status
  .env.example
firmware/
  main.py                   WiFi + polling loop + display
  config.example.py         WiFi/backend settings (copy to config.py)
  lib/wifi.py               WiFi helper
  lib/epd3in7.py            Waveshare Pico-ePaper-3.7 driver (4-gray)
```

## 1. Telegram bot setup

1. Talk to [@BotFather](https://t.me/BotFather), `/newbot`, and copy the **token**.
2. Add the bot as an **administrator** of your channel. A bot can only receive
   `channel_post` updates from channels where it is an admin.
3. Identify the channel:
   - Public channel: use its `@username`.
   - Private channel: use the numeric id (e.g. `-1001234567890`). Easiest way to
     find it: post in the channel, then open
     `https://api.telegram.org/bot<TOKEN>/getUpdates` in a browser and read
     `channel_post.chat.id`.

## 2. Run the backend

Requires Go 1.22+.

```bash
cd backend
cp .env.example .env        # edit TELEGRAM_BOT_TOKEN and CHANNEL_ID
set -a && . ./.env && set +a
go run ./cmd/server
```

Open <http://localhost:8080/> to see a live preview of what the display will
show. Endpoints:

| Endpoint     | Purpose                                                      |
|--------------|-------------------------------------------------------------|
| `/frame.bin` | Raw 33600-byte 4-gray buffer. Honors `If-None-Match` (304). |
| `/frame.png` | PNG preview (4 shades), for debugging in a browser.         |
| `/status`    | JSON: channel, message count, current ETag, last update.    |

Configuration (all via environment, see `.env.example`):
`TELEGRAM_BOT_TOKEN`, `CHANNEL_ID`, `POLL_INTERVAL`, `HTTP_ADDR`,
`MESSAGES_TO_SHOW`, `STORE_PATH`.

For production, build a binary and run it under your process manager:

```bash
go build -o telegram-epaper-display ./cmd/server
```

## 3. Flash the Pico W

1. Install **MicroPython** for the Pico W (the firmware uses the `network`
   module). Drag the official `.uf2` onto the Pico in BOOTSEL mode.
2. Wire the Pico-ePaper-3.7 to the Pico W (it plugs directly onto the Pico
   headers; the bundled driver uses the standard Waveshare pin map).
3. Copy the firmware to the Pico (e.g. with
   [`mpremote`](https://docs.micropython.org/en/latest/reference/mpremote.html)
   or Thonny):

   ```bash
   cd firmware
   cp config.example.py config.py     # edit WiFi + BACKEND_URL
   mpremote connect auto fs cp config.py :config.py
   mpremote connect auto fs cp main.py :main.py
   mpremote connect auto fs mkdir lib
   mpremote connect auto fs cp lib/wifi.py :lib/wifi.py
   mpremote connect auto fs cp lib/epd3in7.py :lib/epd3in7.py
   ```

   Set `BACKEND_URL` to your server's `/frame.bin`, e.g.
   `http://192.168.1.10:8080/frame.bin`.

4. Reset the Pico. It connects to WiFi, fetches the frame, and displays it; then
   it polls every `POLL_INTERVAL_S` seconds, redrawing only when the content
   changes (the backend returns `304` otherwise).

### Display orientation

The backend renders in landscape and rotates 90° clockwise into the panel's
native portrait buffer. If the image appears upside down for how your device is
mounted, flip the rotation in `backend/internal/render/pack.go` (`Pack`): swap
to `lx = (CanvasW-1)-py` / `ly = px` (rotate the other way). No firmware change
needed.

## Verification

- **Backend render**: `cd backend && go test ./...` (checks buffer size and the
  `304` behavior). To eyeball the layout:
  `DUMP_PREVIEW=/tmp/preview.png go test ./internal/render -run Preview`.
- **End to end (server)**: run the backend, post in the channel, watch `/status`
  update and `/frame.png` change.
- **End to end (device)**: power the Pico W, confirm the console shows
  `wifi: connected` and `displayed frame`, then post a new message and watch the
  e-paper update on the next poll.

## Notes / future work

- Emoji are not rendered in v1 (4-gray e-paper + monochrome font); they show as
  missing glyphs. A monochrome emoji font could be added later.
- v1 uses Bot API long polling (no public HTTPS required). A webhook would need
  a public TLS endpoint.
- Aggressive battery saving (panel deep-sleep + RTC deep sleep between polls) is
  out of scope for v1; the firmware uses `machine.lightsleep` between polls.

The bundled `firmware/lib/epd3in7.py` is the Waveshare driver, under its
original MIT-style license (header kept intact).
```
