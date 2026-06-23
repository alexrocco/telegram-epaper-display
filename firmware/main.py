"""telegram-epaper-display firmware for Raspberry Pi Pico W + Waveshare Pico-ePaper-3.7.

Polls the backend's /frame.bin endpoint and pushes the 1-bit buffer straight to
the e-paper using the panel's sharp 1Gray full-refresh mode. Uses ETag /
If-None-Match so the panel only redraws when the content actually changed.
"""

import gc
import socket
import sys
import time

sys.path.append("lib")  # make lib/ modules importable on MicroPython

import config
import wifi
import epd3in7

# Exact size of the 1-bit framebuffer (280 * 480 / 8). The backend serves this
# verbatim; anything else means a bad/partial response.
EXPECTED_BYTES = 280 * 480 // 8


def parse_url(url):
    """Split an http://host[:port]/path URL into (host, port, path)."""
    if not url.startswith("http://"):
        raise ValueError("only http:// URLs are supported: " + url)
    rest = url[len("http://"):]
    slash = rest.find("/")
    if slash == -1:
        hostport, path = rest, "/"
    else:
        hostport, path = rest[:slash], rest[slash:]
    if ":" in hostport:
        host, port = hostport.split(":", 1)
        port = int(port)
    else:
        host, port = hostport, 80
    return host, port, path


def fetch_frame(url, etag, buf):
    """GET url, sending If-None-Match if etag is set.

    On a 200 response the body is read directly into buf (a preallocated
    bytearray, the e-paper framebuffer) to avoid large temporary allocations on
    the memory-constrained Pico. Returns (status, new_etag, nbytes) where nbytes
    is the number of body bytes written into buf (0 for 304).
    """
    host, port, path = parse_url(url)
    addr = socket.getaddrinfo(host, port)[0][-1]
    s = socket.socket()
    s.settimeout(20)
    try:
        s.connect(addr)
        req = "GET %s HTTP/1.0\r\nHost: %s\r\nConnection: close\r\n" % (path, host)
        if etag:
            req += "If-None-Match: %s\r\n" % etag
        req += "\r\n"
        s.send(req.encode())

        # Read just the header (small reads) until the blank line.
        header = b""
        while header.find(b"\r\n\r\n") == -1:
            chunk = s.recv(256)
            if not chunk:
                break
            header += chunk
        sep = header.find(b"\r\n\r\n")
        if sep == -1:
            raise ValueError("malformed HTTP response")

        head = header[:sep].decode()
        leftover = header[sep + 4:]  # body bytes already read with the header

        lines = head.split("\r\n")
        status = int(lines[0].split(" ")[1])
        new_etag = etag
        for ln in lines[1:]:
            if ln.lower().startswith("etag:"):
                new_etag = ln.split(":", 1)[1].strip()

        if status != 200:
            return status, new_etag, 0

        # Stream the body straight into buf: copy what we already have, then
        # readinto the remainder. No big temporary buffers are allocated.
        mv = memoryview(buf)
        n = len(leftover)
        if n > len(buf):
            return status, new_etag, n  # oversized; caller will reject
        mv[0:n] = leftover
        while n < len(buf):
            got = s.readinto(mv[n:])
            if not got:
                break
            n += got
        return status, new_etag, n
    finally:
        s.close()


def sleep():
    ms = config.POLL_INTERVAL_S * 1000
    if getattr(config, "USE_LIGHTSLEEP", False):
        import machine
        machine.lightsleep(ms)
    else:
        time.sleep(config.POLL_INTERVAL_S)


def main():
    # Initialise the display first (no network needed). WiFi is connected inside
    # the loop via wifi.ensure so a connection failure retries instead of
    # crashing the whole program.
    epd = epd3in7.EPD_3in7()       # constructor runs 4Gray init + clear
    epd.EPD_3IN7_1Gray_init()      # switch to the sharp 1-bit full-refresh mode
    epd.EPD_3IN7_1Gray_Clear()
    wlan = None
    etag = None

    while True:
        try:
            gc.collect()
            wlan = wifi.ensure(wlan, config.WIFI_SSID, config.WIFI_PASSWORD)
            status, new_etag, n = fetch_frame(config.BACKEND_URL, etag, epd.buffer_1Gray)
            if status == 200:
                if n == EXPECTED_BYTES:
                    epd.EPD_3IN7_1Gray_Display(epd.buffer_1Gray)
                    etag = new_etag
                    print("displayed frame, etag =", etag)
                else:
                    print("unexpected body length:", n)
            elif status == 304:
                print("not modified")
            else:
                print("unexpected HTTP status:", status)
        except Exception as e:  # noqa: BLE001 - keep the loop alive
            print("poll error:", e)

        sleep()


if __name__ == "__main__":
    main()
