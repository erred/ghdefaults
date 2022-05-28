module go.seankhliao.com/ghdefaults

go 1.19

require (
	github.com/bradleyfalzon/ghinstallation/v2 v2.0.4
	github.com/go-logr/logr v1.2.3
	github.com/google/go-github/v41 v41.0.0
	go.seankhliao.com/svcrunner v0.0.0-20220528093611-00f1089335a6
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
)

require (
	github.com/golang-jwt/jwt/v4 v4.0.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/iand/logfmtr v0.2.1 // indirect
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110 // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
	golang.org/x/text v0.3.3 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/grpc v1.46.2 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
)

retract [v0.1.0, v0.4.1] // old versions
