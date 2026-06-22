"""Copy this file to config.py and fill in your values.

config.py is gitignored so your WiFi password never gets committed.
"""

# WiFi credentials.
WIFI_SSID = "your-wifi-name"
WIFI_PASSWORD = "your-wifi-password"

# Backend frame endpoint (the /frame.bin URL of your server).
BACKEND_URL = "http://192.168.1.10:8080/frame.bin"

# How often to poll the backend, in seconds.
POLL_INTERVAL_S = 30

# Use machine.lightsleep between polls (lower power) instead of time.sleep.
# Note: lightsleep keeps RAM, so the cached ETag survives between cycles.
USE_LIGHTSLEEP = True
