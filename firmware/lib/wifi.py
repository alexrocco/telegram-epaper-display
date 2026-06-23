"""WiFi connection helper for the Pico W."""

import network
import time


def connect(ssid, password, timeout_s=45):
    """Connect to WiFi, returning the active WLAN once it has an IP.

    Raises RuntimeError if the connection is not established within timeout_s.
    Callers should retry rather than treat this as fatal.
    """
    wlan = network.WLAN(network.STA_IF)
    wlan.active(True)
    # Disable WiFi power management: the Pico W's default power-save mode causes
    # slow/flaky connections and dropped links for always-on polling.
    try:
        wlan.config(pm=0xA11140)
    except Exception:
        pass

    if not wlan.isconnected():
        print("wifi: connecting to", ssid)
        wlan.connect(ssid, password)
        deadline = time.ticks_add(time.ticks_ms(), timeout_s * 1000)
        while not wlan.isconnected():
            if time.ticks_diff(deadline, time.ticks_ms()) <= 0:
                raise RuntimeError("wifi: connection timed out (status %d)" % wlan.status())
            time.sleep_ms(250)
    print("wifi: connected, ip =", wlan.ifconfig()[0])
    return wlan


def ensure(wlan, ssid, password, timeout_s=45):
    """Reconnect wlan if the link dropped. Returns a connected WLAN."""
    if wlan is not None and wlan.isconnected():
        return wlan
    return connect(ssid, password, timeout_s)
