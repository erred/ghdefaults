package main

import (
	"context"
	"encoding/base64"
	"flag"
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.seankhliao.com/usvc"
)

var (
	name        = "go.seankhliao.com/ghdefaults"
	AppID int64 = 62448
)

func main() {
	os.Exit(usvc.Exec(context.Background(), &Server{}, os.Args))
}

type Server struct {
	WebHookSecret string
	privBase64    string
	pkey          []byte

	log    zerolog.Logger
	tracer trace.Tracer
}

func (s *Server) Flags(fs *flag.FlagSet) {
	fs.StringVar(&s.WebHookSecret, "webhook-secret", os.Getenv("WEBHOOK_SECRET"), "webhook validation secret")
	fs.StringVar(&s.privBase64, "priv", os.Getenv("PRIVATE_KEY"), "base64 encoded private key")
}

func (s *Server) Setup(ctx context.Context, c *usvc.USVC) error {
	s.log = c.Logger
	s.tracer = global.Tracer(name)

	var err error
	s.pkey, err = base64.StdEncoding.DecodeString(s.privBase64)
	if err != nil {
		s.log.Error().Err(err).Msg("decode private key")
	}

	c.ServiceMux.Handle("/", s)
	return nil
}

func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.tracer.Start(r.Context(), "serve")
	defer span.End()

	payload, err := github.ValidatePayload(r, []byte(s.WebHookSecret))
	if err != nil {
		s.log.Error().Err(err).Msg("webhook validation failed")
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		s.log.Error().Err(err).Msg("webhook parse failed")
		return
	}

	switch event := event.(type) {
	case *github.RepositoryEvent:
		if *event.Action != "created" {
			break
		}
		repo := &github.Repository{
			HasIssues:           github.Bool(false),
			HasWiki:             github.Bool(false),
			HasProjects:         github.Bool(false),
			AllowMergeCommit:    github.Bool(false),
			AllowRebaseMerge:    github.Bool(false),
			DeleteBranchOnMerge: github.Bool(true),
		}

		itr, err := ghinstallation.New(
			http.DefaultTransport, AppID, *event.Installation.ID, s.pkey)
		if err != nil {
			s.log.Error().Err(err).Msg("create gh installation")
			return
		}
		client := github.NewClient(&http.Client{Transport: itr})

		o := *event.Repo.Owner.Login
		re := *event.Repo.Name
		ctx, span := s.tracer.Start(ctx, "edit repo")
		defer span.End()
		_, _, err = client.Repositories.Edit(ctx, o, re, repo)
		if err != nil {
			s.log.Error().Err(err).Str("owner", o).Str("repo", re).Msg("edit repository")
			return
		}
		s.log.Info().Str("owner", o).Str("repo", re).Msg("defaults set")
	}
}
