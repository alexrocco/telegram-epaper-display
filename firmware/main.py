"""telegram-epaper-display firmware for Raspberry Pi Pico W + Waveshare Pico-ePaper-3.7.

Polls the backend's /frame.bin endpoint and pushes the 4-gray buffer straight to
the e-paper. Uses ETag / If-None-Match so the panel only redraws when the
content actually changed.
"""

import socket
import sys
import time

sys.path.append("lib")  # make lib/ modules importable on MicroPython

import config
import wifi
import epd3in7

# Exact size of the 4-gray framebuffer (280 * 480 / 4). The backend serves this
# verbatim; anything else means a bad/partial response.
EXPECTED_BYTES = 280 * 480 // 4


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


def fetch_frame(url, etag):
    """GET url, sending If-None-Match if etag is set.

    Returns (status, new_etag, body_bytes). body_bytes is b"" for 304.
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

        chunks = []
        while True:
            chunk = s.recv(1024)
            if not chunk:
                break
            chunks.append(chunk)
    finally:
        s.close()

    raw = b"".join(chunks)
    sep = raw.find(b"\r\n\r\n")
    if sep == -1:
        raise ValueError("malformed HTTP response")
    head = raw[:sep].decode()
    body = raw[sep + 4:]

    lines = head.split("\r\n")
    status = int(lines[0].split(" ")[1])
    new_etag = etag
    for ln in lines[1:]:
        if ln.lower().startswith("etag:"):
            new_etag = ln.split(":", 1)[1].strip()
    return status, new_etag, body


def display(epd, body):
    """Push the received GS2_HMSB buffer to the panel."""
    epd.buffer_4Gray[:] = body
    epd.EPD_3IN7_4Gray_Display(epd.buffer_4Gray)


def sleep():
    ms = config.POLL_INTERVAL_S * 1000
    if getattr(config, "USE_LIGHTSLEEP", False):
        import machine
        machine.lightsleep(ms)
    else:
        time.sleep(config.POLL_INTERVAL_S)


def main():
    wlan = wifi.connect(config.WIFI_SSID, config.WIFI_PASSWORD)
    epd = epd3in7.EPD_3in7()  # constructor runs 4Gray init + clear
    etag = None

    while True:
        try:
            wlan = wifi.ensure(wlan, config.WIFI_SSID, config.WIFI_PASSWORD)
            status, new_etag, body = fetch_frame(config.BACKEND_URL, etag)
            if status == 200:
                if len(body) == EXPECTED_BYTES:
                    display(epd, body)
                    etag = new_etag
                    print("displayed frame, etag =", etag)
                else:
                    print("unexpected body length:", len(body))
            elif status == 304:
                print("not modified")
            else:
                print("unexpected HTTP status:", status)
        except Exception as e:  # noqa: BLE001 - keep the loop alive
            print("poll error:", e)

        sleep()


if __name__ == "__main__":
    main()
