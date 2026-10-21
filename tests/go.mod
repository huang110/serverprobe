module github.com/huang110/serverprobe/tests

go 1.21

require (
	github.com/alicebob/miniredis/v2 v2.33.0
	github.com/gorilla/websocket v1.5.3
	github.com/huang110/serverprobe/server v0.0.0
	google.golang.org/grpc v1.62.1
)

require (
	github.com/alicebob/gopher-json v0.0.0-20200520072559-a9ecdc9d1d3a // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240314234333-6e1732d8331c // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)

replace github.com/huang110/serverprobe/server => ../server
