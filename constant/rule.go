package constant

// Rule Type
const (
	Domain RuleType = iota
	DomainSuffix
	DomainKeyword
	GEOIP
	IPCIDR
	SrcIPCIDR
	SrcPort
	DstPort
	Process
	MATCH
	ALLIP
	DomainDstPort
	DomainSrcPort
	//SrcIPPort
	DstIPPort
)

type RuleType int

func (rt RuleType) String() string {
	switch rt {
	case Domain:
		return "DOMAIN"
	case DomainSuffix:
		return "DOMAIN-SUFFIX"
	case DomainKeyword:
		return "DOMAIN-KEYWORD"
	case GEOIP:
		return "GEOIP"
	case IPCIDR:
		return "IP-CIDR"
	case SrcIPCIDR:
		return "SRC-IP-CIDR"
	case SrcPort:
		return "SRC-PORT"
	case DstPort:
		return "DST-PORT"
	case Process:
		return "PROCESS-NAME"
	case MATCH:
		return "MATCH"
	case ALLIP:
		return "ALLIP"
	case DomainDstPort:
		return "DOMAIN-SRC-PORT"
	case DomainSrcPort:
		return "DOMAIN-DST-PORT"
	//case SrcIPPort:
	//	return "SRC-IP-PORT"
	case DstIPPort:
		return "DST-IP-PORT"
	default:
		return "Unknown"
	}
}

type Rule interface {
	RuleType() RuleType
	Match(metadata *Metadata) bool
	Adapter() string
	Payload() string
	ShouldResolveIP() bool
	MultiDomainDialIP() bool
}
