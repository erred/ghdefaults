package main

import (
	"encoding/base64"
	"flag"
	"net/http"
	"os"

	"github.com/google/go-github/v31/github"
	"go.seankhliao.com/usvc"

	"github.com/bradleyfalzon/ghinstallation"
)

var (
	AppID int64 = 62448
)

func main() {
	s := NewServer(os.Args)
	s.svc.Log.Error().Err(usvc.Run(usvc.SignalContext(), s.svc)).Msg("exited")
}

type Server struct {
	WebHookSecret string
	pkey          []byte

	// server
	svc *usvc.ServerSimple
}

func NewServer(args []string) *Server {
	fs := flag.NewFlagSet(args[0], flag.ExitOnError)
	s := &Server{
		svc: usvc.NewServerSimple(usvc.NewConfig(fs)),
	}

	s.svc.Mux.Handle("/", http.RedirectHandler("https://github.com/seankhliao/github-defaults", http.StatusFound))
	s.svc.Mux.Handle("/webhook", s)

	var priv string
	flag.StringVar(&s.WebHookSecret, "webhook-secret", os.Getenv("WEBHOOK_SECRET"), "webhook validation secret")
	flag.StringVar(&priv, "priv", os.Getenv("PRIVATE_KEY"), "base64 encoded private key")
	fs.Parse(args[1:])

	var err error
	s.pkey, err = base64.StdEncoding.DecodeString(priv)
	if err != nil {
		s.svc.Log.Fatal().Err(err).Msg("decode private key")
	}
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(s.WebHookSecret))
	if err != nil {
		s.svc.Log.Debug().Err(err).Msg("webhook validation failed")
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		s.svc.Log.Debug().Err(err).Msg("webhook parse failed")
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
			s.svc.Log.Error().Err(err).Msg("create gh installation")
			return
		}
		client := github.NewClient(&http.Client{Transport: itr})

		o := *event.Repo.Owner.Login
		re := *event.Repo.Name

		_, _, err = client.Repositories.Edit(r.Context(), o, re, repo)
		if err != nil {
			s.svc.Log.Error().Err(err).Str("owner", o).Str("repo", re).Msg("edit repository")
			return
		}
		s.svc.Log.Info().Str("owner", o).Str("repo", re).Msg("defaults set")
	}
}
