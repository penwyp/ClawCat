package cache

import (
	"github.com/bytedance/sonic"
)

// SonicSerializer implements Serializer using ByteDance Sonic JSON library
type SonicSerializer struct {
	api sonic.API
}

// NewSonicSerializer creates a new Sonic-based serializer
func NewSonicSerializer() *SonicSerializer {
	return &SonicSerializer{
		api: sonic.ConfigDefault,
	}
}

// Serialize converts a value to bytes using Sonic JSON
func (s *SonicSerializer) Serialize(v interface{}) ([]byte, error) {
	return s.api.Marshal(v)
}

// Deserialize converts bytes back to a value using Sonic JSON
func (s *SonicSerializer) Deserialize(data []byte, v interface{}) error {
	return s.api.Unmarshal(data, v)
}
