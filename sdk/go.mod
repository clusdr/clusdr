module github.com/clusdr/clusdr/sdk

go 1.27.0

require (
	github.com/clusdr/clusdr/api v0.2.0
	google.golang.org/grpc v1.84.0
)

require (
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/clusdr/clusdr/api => ../api
