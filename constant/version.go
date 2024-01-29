package constant

import "runtime"

var (
	Version     = "unknown version"
	BuildTime   = "unknown time"
	UNSaveDNSDB = runtime.GOOS == "linux" && runtime.GOARCH == "arm64"
)
