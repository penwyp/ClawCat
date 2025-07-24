package cache

import (
	"bytes"
	"compress/gzip"
	"io"

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

// CompressedSerializer wraps another serializer with compression
type CompressedSerializer struct {
	serializer Serializer
	level      int
}

// NewCompressedSerializer creates a new compressed serializer
func NewCompressedSerializer(s Serializer, level int) *CompressedSerializer {
	if level < gzip.BestSpeed || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}
	
	return &CompressedSerializer{
		serializer: s,
		level:      level,
	}
}

// Serialize compresses the serialized data
func (c *CompressedSerializer) Serialize(v interface{}) ([]byte, error) {
	// First serialize with underlying serializer
	data, err := c.serializer.Serialize(v)
	if err != nil {
		return nil, err
	}
	
	// Then compress
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, err
	}
	
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, err
	}
	
	if err := writer.Close(); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

// Deserialize decompresses and deserializes the data
func (c *CompressedSerializer) Deserialize(data []byte, v interface{}) error {
	// First decompress
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer reader.Close()
	
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	
	// Then deserialize with underlying serializer
	return c.serializer.Deserialize(decompressed, v)
}