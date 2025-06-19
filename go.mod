module file-upload-server

go 1.23.0

toolchain go1.23.10

require (
	github.com/cloudwego/hertz v0.8.0
	github.com/hertz-contrib/cors v0.1.0
	tos_tool v0.0.0
	viking_db_tool v0.0.0
)

replace tos_tool => ./tos_tool

replace viking_db_tool => ./viking_db_tool

require (
	github.com/bytedance/go-tagexpr/v2 v2.9.2 // indirect
	github.com/bytedance/gopkg v0.0.0-20230728082804-614d0af6619b // indirect
	github.com/bytedance/sonic v1.13.2 // indirect
	github.com/bytedance/sonic/loader v0.2.4 // indirect
	github.com/cenkalti/backoff/v4 v4.1.2 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/cloudwego/netpoll v0.5.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/henrylee2cn/ameda v1.4.10 // indirect
	github.com/henrylee2cn/goutil v0.0.0-20210127050712-89660552f6f8 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/nyaruka/phonenumbers v1.0.55 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/volcengine/ve-tos-golang-sdk/v2 v2.7.15 // indirect
	github.com/volcengine/volc-sdk-golang v1.0.212 // indirect
	golang.org/x/arch v0.11.0 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
)
