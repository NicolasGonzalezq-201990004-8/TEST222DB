module michael

go 1.21

require (
	franklin v0.0.0
	google.golang.org/grpc v1.66.0
	lester v0.0.0-00010101000000-000000000000
	trevor v0.0.0
)

require (
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240604185151-ef581f913117 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

replace lester => ../lester

replace franklin => ../franklin

replace trevor => ../trevor
