package vpn

import (
	"fmt"
	"sync"
)

// Chunk представляет собой кусок передаваемого файла или сообщения
type Chunk struct {
	ID     string // Уникальный идентификатор файла/стрима
	Offset int64  // Смещение в байтах
	Data   []byte // Полезная нагрузка (до 64KB для оптимизации WebTransport)
	IsLast bool   // Флаг последнего чанка
}

// UploadSession отслеживает состояние загрузки (для поддержки докачки)
type UploadSession struct {
	ID            string
	ReceivedBytes int64
	Chunks        map[int64][]byte
	mu            sync.Mutex
	IsComplete    bool
}

// ResumableManager управляет всеми активными P2P загрузками
type ResumableManager struct {
	sessions map[string]*UploadSession
	mu       sync.RWMutex
}

// NewResumableManager создает новый менеджер
func NewResumableManager() *ResumableManager {
	return &ResumableManager{
		sessions: make(map[string]*UploadSession),
	}
}

// InitSession инициирует новую сессию или возвращает текущее смещение (offset) для докачки
func (rm *ResumableManager) InitSession(id string) int64 {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if session, exists := rm.sessions[id]; exists {
		// Сессия уже существует, возвращаем сколько байт мы УЖЕ получили
		return session.ReceivedBytes
	}

	// Новая сессия
	rm.sessions[id] = &UploadSession{
		ID:            id,
		Chunks:        make(map[int64][]byte),
		ReceivedBytes: 0,
	}
	return 0
}

// ProcessChunk добавляет новый чанк в буфер (обрабатывая сетевой хаос и дубликаты)
func (rm *ResumableManager) ProcessChunk(chunk Chunk) error {
	rm.mu.RLock()
	session, exists := rm.sessions[chunk.ID]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found. Call InitSession first", chunk.ID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Идемпотентность: Защита от дублирования чанков при сетевых ретраях
	if _, ok := session.Chunks[chunk.Offset]; ok {
		return nil 
	}

	session.Chunks[chunk.Offset] = chunk.Data
	session.ReceivedBytes += int64(len(chunk.Data))

	if chunk.IsLast {
		session.IsComplete = true
		// TODO: Сборка файла из чанков и передача в шифрованное хранилище Vault
	}

	return nil
}
