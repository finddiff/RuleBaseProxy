package rules

import (
	C "github.com/finddiff/clashWithCache/constant"
	"github.com/finddiff/clashWithCache/log"
)

type AllIP struct {
	adapter string
}

func (f *AllIP) RuleType() C.RuleType {
	return C.ALLIP
}

func (f *AllIP) Match(metadata *C.Metadata) bool {
	//log.Debugln("host:%v| DstIP:%v| SrcIP:%v",metadata.Host,metadata.DstIP,metadata.SrcIP)
	//ip := metadata.DstIP
	if len(metadata.Host) > 1 {
		log.Debugln("allip match false host:%v", metadata.Host)
		return false
	}
	log.Debugln("allip match %v", metadata.DstIP != nil || metadata.SrcIP != nil)
	return metadata.DstIP != nil || metadata.SrcIP != nil
}

func (f *AllIP) Adapter() string {
	return f.adapter
}

func (f *AllIP) Payload() string {
	return ""
}

func (f *AllIP) ShouldResolveIP() bool {
	return false
}

func NewAllIP(adapter string) *AllIP {
	return &AllIP{
		adapter: adapter,
	}
}
