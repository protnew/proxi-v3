package chat

import "errors"

var (
	// ErrNotBinaryVoice is returned when a binary frame is not a voice message.
	ErrNotBinaryVoice = errors.New("not a binary voice frame")

	// ErrInvalidBinaryVoice is returned when a binary voice frame has invalid format.
	ErrInvalidBinaryVoice = errors.New("invalid binary voice frame format")
)
