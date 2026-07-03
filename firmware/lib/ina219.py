"""Minimal INA219 driver for the Waveshare Pico-UPS-B.

The UPS-B carries an INA219 current/voltage monitor on I2C address 0x43, wired
to the Pico's I2C1 bus (SDA=GP6, SCL=GP7). These pins don't clash with the
Pico-ePaper-3.7, which is SPI (see lib/epd3in7.py).

Only what the firmware needs is implemented: bus voltage (battery volts) and
current (signed mA, positive while charging). Calibration matches Waveshare's
Pico-UPS-B example (16V / 5A range, 0.1 ohm shunt).
"""

from machine import I2C, Pin

_REG_CONFIG = 0x00
_REG_BUSVOLTAGE = 0x02
_REG_CURRENT = 0x04
_REG_CALIBRATION = 0x05


class INA219:
    def __init__(self, i2c_bus=1, sda=6, scl=7, addr=0x43, freq=100000):
        self.i2c = I2C(i2c_bus, sda=Pin(sda), scl=Pin(scl), freq=freq)
        self.addr = addr
        # 16V / 5A calibration (0.1 ohm shunt): current LSB = 0.1524 mA/bit.
        self._cal_value = 26868
        self._current_lsb = 0.1524
        self._configure()

    def _write(self, reg, value):
        self.i2c.writeto_mem(self.addr, reg, bytes([(value >> 8) & 0xFF, value & 0xFF]))

    def _read(self, reg):
        data = self.i2c.readfrom_mem(self.addr, reg, 2)
        return (data[0] << 8) | data[1]

    def _configure(self):
        self._write(_REG_CALIBRATION, self._cal_value)
        # BusVoltageRange 16V (0) | Gain /2 80mV (1) | 12-bit 32-sample bus &
        # shunt ADC (0x0D each) | shunt+bus continuous mode (7).
        config = (0x00 << 13) | (0x01 << 11) | (0x0D << 7) | (0x0D << 3) | 0x07
        self._write(_REG_CONFIG, config)

    def bus_voltage(self):
        """Battery voltage in volts."""
        # Re-arm calibration in case the chip was power-cycled independently.
        self._write(_REG_CALIBRATION, self._cal_value)
        raw = self._read(_REG_BUSVOLTAGE)
        return (raw >> 3) * 0.004

    def current_mA(self):
        """Current in mA; positive while charging, negative while discharging."""
        raw = self._read(_REG_CURRENT)
        if raw > 32767:  # two's complement for 16-bit signed
            raw -= 65536
        return raw * self._current_lsb
