package statistic

import (
	"github.com/dgraph-io/ristretto"
)

var (
	//ManagerTimeout     = CC.New(CC.Configure().MaxSize(1024 * 1024).ItemsToPrune(500).Buckets(1024 * 1024 / 64))
	//ManagerReadTimeout = CC.New(CC.Configure().MaxSize(1024 * 1024).ItemsToPrune(500).Buckets(1024 * 1024 / 64))
	ManagerTimeout, _ = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 26, // maximum cost of cache (64MB).
		BufferItems: 64,      // number of keys per Get buffer.
	})

	ManagerReadTimeout, _ = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 26, // maximum cost of cache (64MB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
)
