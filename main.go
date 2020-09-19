package main

import (
	"context"
	"encoding/base64"
	"flag"
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
	"github.com/rs/zerolog"
	"go.seankhliao.com/usvc"
)

var (
	AppID int64 = 62448
)

func main() {
	var srvconf usvc.Conf
	var s Server

	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	srvconf.RegisterFlags(fs)
	s.RegisterFlags(fs)
	fs.Parse(os.Args[1:])

	s.log = zerolog.New(os.Stdout).With().Timestamp().Logger()

	var err error
	s.pkey, err = base64.StdEncoding.DecodeString(s.privBase64)
	if err != nil {
		s.log.Error().Err(err).Msg("decode private key")
	}

	m := http.NewServeMux()
	m.Handle("/", s)

	_, run, err := srvconf.Server(m, s.log)
	if err != nil {
		s.log.Error().Err(err).Msg("prepare server")
		os.Exit(1)
	}

	err = run(context.Background())
	if err != nil {
		s.log.Error().Err(err).Msg("exit")
		os.Exit(1)
	}
}

type Server struct {
	WebHookSecret string
	privBase64    string
	pkey          []byte

	log zerolog.Logger
}

func (s *Server) RegisterFlags(fs *flag.FlagSet) {
	flag.StringVar(&s.WebHookSecret, "webhook-secret", os.Getenv("WEBHOOK_SECRET"), "webhook validation secret")
	flag.StringVar(&s.privBase64, "priv", os.Getenv("PRIVATE_KEY"), "base64 encoded private key")
}

func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

		_, _, err = client.Repositories.Edit(r.Context(), o, re, repo)
		if err != nil {
			s.log.Error().Err(err).Str("owner", o).Str("repo", re).Msg("edit repository")
			return
		}
		s.log.Info().Str("owner", o).Str("repo", re).Msg("defaults set")
	}
}
