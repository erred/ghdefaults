module go.seankhliao.com/ghdefaults

go 1.19

require (
	github.com/bradleyfalzon/ghinstallation/v2 v2.0.4
	github.com/go-logr/logr v1.2.3
	github.com/google/go-github/v41 v41.0.0
	go.seankhliao.com/svcrunner v0.1.3
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
)

require (
	github.com/golang-jwt/jwt/v4 v4.0.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/iand/logfmtr v0.2.1 // indirect
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/net v0.0.0-20220531201128-c960675eff93 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220531173845-685668d2de03 // indirect
	google.golang.org/grpc v1.47.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
)

retract [v0.1.0, v0.4.1] // old versions
