// LEGACY / dead: not the live WT server. Live code is src/src-vpn/webtransport_server.go. Do not wire.
package main

import (
	"log"
	"vpn"
)

func main() {
	// Инициализация WebTransport VPN сервера на UDP порту 4433 (стандартный для QUIC/HTTP3)
	server := vpn.NewServer("127.0.0.1:4433")
	
	log.Println("Starting WebTransport VPN Server on localhost:4433...")
	// Запуск с самоподписанными сертификатами (cert.pem, cert.key)
	err := server.Start("cert.pem", "cert.key")
	if err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
}
