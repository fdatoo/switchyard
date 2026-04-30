package api

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
)

const (
	DefaultPageSize = 100
	MaxPageSize     = 1000
)

type Cursor struct {
	Position uint64
	Tiebreak string
}

func EncodeCursor(c Cursor) (string, error) {
	if c.Position == 0 && c.Tiebreak == "" {
		return "", nil
	}
	buf := make([]byte, 8+len(c.Tiebreak))
	binary.BigEndian.PutUint64(buf[:8], c.Position)
	copy(buf[8:], c.Tiebreak)
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func DecodeCursor(token string) (Cursor, error) {
	if token == "" {
		return Cursor{}, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, err
	}
	if len(b) < 8 {
		return Cursor{}, errors.New("cursor: short token")
	}
	return Cursor{
		Position: binary.BigEndian.Uint64(b[:8]),
		Tiebreak: string(b[8:]),
	}, nil
}

func ClampPageSize(n uint32) uint32 {
	switch {
	case n == 0:
		return DefaultPageSize
	case n > MaxPageSize:
		return MaxPageSize
	default:
		return n
	}
}
