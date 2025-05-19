module clash-test

go 1.21.4

toolchain go1.22.1

require (
	github.com/docker/docker v20.10.8+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/finddiff/RuleBaseProxy v1.6.6-0.20210905062555-c7b718f6512d
	github.com/miekg/dns v1.1.58
	github.com/stretchr/testify v1.8.4
	golang.org/x/net v0.20.0
)

replace github.com/finddiff/RuleBaseProxy => ../

require (
	github.com/Dreamacro/go-shadowsocks2 v0.1.8 // indirect
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/containerd/containerd v1.5.5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20240129002554-15c9b8791914 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/oschwald/geoip2-golang v1.9.0 // indirect
	github.com/oschwald/maxminddb-golang v1.11.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/u-root/uio v0.0.0-20230220225925-ffce2a382923 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/genproto v0.0.0-20201110150050-8816d57aaa9a // indirect
	google.golang.org/grpc v1.40.0 // indirect
	google.golang.org/protobuf v1.26.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
