// LEGACY / dead: not the live WT server. Live code is src/src-vpn/webtransport_server.go. Do not wire.
package vpn

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

// Server инкапсулирует WebTransport VPN сервер.
type Server struct {
	addr string
	wt   *webtransport.Server
}

// NewServer создает новый инстанс WebTransport сервера
func NewServer(addr string) *Server {
	s := &Server{
		addr: addr,
	}

	// Инициализация мультиплексора WebTransport поверх HTTP/3
	s.wt = &webtransport.Server{
		H3: http3.Server{
			Addr: addr,
		},
		// Разрешаем подключения с любых Origin (в проде нужно проверять список доверенных доменов)
		CheckOrigin: func(r *http.Request) bool {
			return true 
		},
	}

	return s
}

// Start запускает QUIC listener и начинает принимать VPN туннели
func (s *Server) Start(certFile, keyFile string) error {
	// Роутер для обработки VPN-подключений
	mux := http.NewServeMux()
	mux.HandleFunc("/vpn", s.handleVPN)
	s.wt.H3.Handler = mux

	log.Printf("WebTransport VPN Server is starting on UDP %s", s.addr)
	
	// Настройка TLS (QUIC жестко требует TLS 1.3)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS keys: %w", err)
	}
	s.wt.H3.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3"}, // Идентификатор ALPN для HTTP/3
	}

	// Запуск сервера (блокирующий вызов)
	return s.wt.ListenAndServe()
}

// handleVPN обрабатывает входящие HTTP/3 запросы и апгрейдит их до WebTransport сессии
func (s *Server) handleVPN(w http.ResponseWriter, r *http.Request) {
	session, err := s.wt.Upgrade(w, r)
	if err != nil {
		log.Printf("WebTransport upgrade failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("New WebTransport VPN session from %s", r.RemoteAddr)

	go s.handleSession(session)
}

// handleSession управляет жизненным циклом одного клиентского подключения
func (s *Server) handleSession(session *webtransport.Session) {
	defer session.CloseWithError(0, "VPN session ended")

	// Главный цикл обработки двунаправленных (bidi) потоков внутри одной сессии
	for {
		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			log.Printf("AcceptStream failed (client disconnected?): %v", err)
			return
		}
		
		go s.handleStream(stream)
	}
}

// handleStream обрабатывает логику внутри конкретного потока (например, одного TCP-коннекта)
func (s *Server) handleStream(stream webtransport.Stream) {
	defer stream.Close()
	log.Printf("Opened new VPN stream %d", stream.StreamID())

	buf := make([]byte, 4096)
	for {
		// Чтение IP-пакета или фрейма от клиента
		n, err := stream.Read(buf)
		if err != nil {
			log.Printf("Stream %d read error: %v", stream.StreamID(), err)
			return
		}
		
		// На этапе TDD (Эхо-сервер) просто возвращаем данные обратно клиенту
		msg := buf[:n]
		log.Printf("Stream %d rx: %d bytes", stream.StreamID(), n)
		
		_, err = stream.Write(msg)
		if err != nil {
			log.Printf("Stream %d write error: %v", stream.StreamID(), err)
			return
		}
	}
}
