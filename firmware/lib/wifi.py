"""WiFi connection helper for the Pico W."""

import network
import time


def connect(ssid, password, timeout_s=30):
    """Connect to WiFi, returning the active WLAN once it has an IP.

    Raises RuntimeError if the connection is not established within timeout_s.
    """
    wlan = network.WLAN(network.STA_IF)
    wlan.active(True)
    if not wlan.isconnected():
        print("wifi: connecting to", ssid)
        wlan.connect(ssid, password)
        deadline = time.ticks_add(time.ticks_ms(), timeout_s * 1000)
        while not wlan.isconnected():
            if time.ticks_diff(deadline, time.ticks_ms()) <= 0:
                raise RuntimeError("wifi: connection timed out")
            time.sleep_ms(250)
    print("wifi: connected, ip =", wlan.ifconfig()[0])
    return wlan


def ensure(wlan, ssid, password, timeout_s=30):
    """Reconnect wlan if the link dropped. Returns a connected WLAN."""
    if wlan is not None and wlan.isconnected():
        return wlan
    return connect(ssid, password, timeout_s)
