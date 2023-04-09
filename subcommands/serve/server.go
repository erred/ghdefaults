package serve

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.seankhliao.com/svcrunner/v2/observability"
	"go.seankhliao.com/svcrunner/v2/tshttp"
)

type Server struct {
	o *observability.O

	svr *tshttp.Server

	webhookSecret string
	privateKey    string
	appID         int64
}

func New(ctx context.Context, c *Cmd) *Server {
	svr := tshttp.New(ctx, c.tshttp)
	s := &Server{
		o:   svr.O,
		svr: svr,

		webhookSecret: c.webhookSecret,
		privateKey:    c.privateKey + "\n",
		appID:         c.appID,
	}

	svr.Mux.Handle("/webhook", otelhttp.NewHandler(http.HandlerFunc(s.hWebhook), "hWebhook"))
	svr.Mux.HandleFunc("/-/ready", func(rw http.ResponseWriter, r *http.Request) { rw.Write([]byte("ok")) })

	return s
}

func (s *Server) Run(ctx context.Context) error {
	return s.svr.Run(ctx)
}
