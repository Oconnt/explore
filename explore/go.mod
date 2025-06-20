module explore

go 1.23.0

toolchain go1.23.9

require (
	github.com/derekparker/trie v0.0.0-20230829180723-39f4de51ef7d
	//github.com/go-delve/delve v1.24.2
	github.com/go-delve/liner v1.2.3-0.20231231155935-4726ab1d7f62
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/mattn/go-colorable v0.1.13
	github.com/mattn/go-isatty v0.0.20
	github.com/urfave/cli v1.22.16
	golang.org/x/sys v0.33.0
	google.golang.org/grpc v1.73.0
)

require (
	github.com/google/uuid v1.6.0
	github.com/hashicorp/golang-lru v1.0.2
	go.uber.org/zap v1.27.0
	golang.org/x/arch v0.11.0
	golang.org/x/telemetry v0.0.0-20241106142447-58a1122356f5
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

//replace github.com/go-delve/delve v1.24.2 => ../delve
