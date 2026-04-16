package core

import "fmt"

// DefaultTokenCounter returns rough estimate at ~4 bytes per token.
func DefaultTokenCounter(size int64, content interface{}) int64 {
	var targetSize int64 = size
	switch v := content.(type) {
	case string:
		targetSize = int64(len(v))
	case []byte:
		targetSize = int64(len(v))
	case fmt.Stringer:
		targetSize = int64(len(v.String()))
	}
	return targetSize / 4
}
