package lx

import "fmt"

type TokenCounter func(size int64, content interface{}) int64

type Tokenizer interface {
	Estimate(size int64, content interface{}) int64
}

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
