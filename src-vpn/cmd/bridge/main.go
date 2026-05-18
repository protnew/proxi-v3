package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"

    vpn "github.com/unkillable-messenger/vpn"
)

// IPC bridge: читает JSON-RPC из stdin, пишет в stdout
// Запускается Tauri как дочерний процесс

func main() {
    mgr, err := vpn.NewManager("")
    if err != nil {
        fmt.Fprintf(os.Stderr, "VPN init error: %v\n", err)
        os.Exit(1)
    }

    // Сигнализируем что готовы
    send(map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "ready",
        "params": map[string]string{
            "publicKey": mgr.GetPublicKey(),
        },
    })

    scanner := bufio.NewScanner(os.Stdin)
    scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

    for scanner.Scan() {
        line := scanner.Bytes()
        if len(line) == 0 {
            continue
        }

        response := mgr.HandleRPC(line)
        os.Stdout.Write(response)
        os.Stdout.Write([]byte("\n"))
    }

    // Cleanup
    mgr.Disconnect()
}

func send(msg map[string]interface{}) {
    data, _ := json.Marshal(msg)
    os.Stdout.Write(data)
    os.Stdout.Write([]byte("\n"))
}
