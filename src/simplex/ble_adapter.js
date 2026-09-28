export class BLEAdapter {
  constructor() {
    this.isConnected = false;
  }

  async startScanning() {
    // Mock Web Bluetooth API
    this.isConnected = true;
    return "DEVICE_FOUND";
  }

  async sendQueueMessage(message) {
    if (!this.isConnected) throw new Error("Not connected to BLE");
    return true; // message sent via BLE L2CAP
  }
}
