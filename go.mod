module github.com/kevin-chtw/tw_client

go 1.24.0

require (
	github.com/kevin-chtw/tw_proto v0.0.0-20250620085309-10bdf0d2fa40
	github.com/sirupsen/logrus v1.9.3
	github.com/topfreegames/pitaya/v3 v3.0.0-beta.6
	google.golang.org/protobuf v1.36.6
)

replace github.com/kevin-chtw/tw_proto => ../tw_proto

require (
	github.com/google/go-cmp v0.6.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
)
