trie.(*DomainTrie).Search (domain.go:94) github.com/finddiff/RuleBaseProxy/component/trie
dns.withHosts.func1.1 (middleware.go:29) github.com/finddiff/RuleBaseProxy/dns
dns.handlerWithContext (server.go:47) github.com/finddiff/RuleBaseProxy/dns
dns.(*Server).ServeDNS (server.go:28) github.com/finddiff/RuleBaseProxy/dns
dns.(*Server).serveDNS (server.go:659) github.com/miekg/dns
dns.(*Server).serveUDPPacket (server.go:603) github.com/miekg/dns
dns.(*Server).serveUDP.func3 (server.go:533) github.com/miekg/dns
runtime.goexit (asm_arm64.s:1172) runtime
- Async Stack Trace
  dns.(*Server).serveUDP (server.go:533) github.com/miekg/dns