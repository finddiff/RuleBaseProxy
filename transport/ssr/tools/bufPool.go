package tools

import (
	"bytes"
	"github.com/finddiff/clashWithCache/common/pool"
	"math/rand"
)

//var BufPool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}

func AppendRandBytes(b *bytes.Buffer, length int) {
	randBytes := pool.Get(length)
	defer pool.Put(randBytes)
	rand.Read(randBytes)
	b.Write(randBytes)
}
