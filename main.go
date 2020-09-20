package main

import (
	"context"
	"encoding/base64"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v32/github"
	"github.com/rs/zerolog"
	"go.seankhliao.com/stream"
	"go.seankhliao.com/usvc"
	"google.golang.org/grpc"
)

var (
	AppID int64 = 62448
)

func main() {
	var s Server

	srvc := usvc.DefaultConf(&s)
	s.log = srvc.Logger()

	var err error
	s.pkey, err = base64.StdEncoding.DecodeString(s.privBase64)
	if err != nil {
		s.log.Error().Err(err).Msg("decode private key")
	}

	cc, err := grpc.Dial(s.streamAddr)
	if err != nil {
		s.log.Error().Err(err).Msg("connect to stream")
	}
	defer cc.Close()
	s.client = stream.NewStreamClient(cc)

	m := http.NewServeMux()
	m.Handle("/", s)

	err = srvc.RunHTTP(context.Background(), m)
	if err != nil {
		s.log.Fatal().Err(err).Msg("run server")
	}
}

type Server struct {
	WebHookSecret string
	privBase64    string
	pkey          []byte

	log zerolog.Logger

	streamAddr string
	client     stream.StreamClient
}

func (s *Server) RegisterFlags(fs *flag.FlagSet) {
	flag.StringVar(&s.WebHookSecret, "webhook-secret", os.Getenv("WEBHOOK_SECRET"), "webhook validation secret")
	flag.StringVar(&s.privBase64, "priv", os.Getenv("PRIVATE_KEY"), "base64 encoded private key")
	fs.StringVar(&s.streamAddr, "stream.addr", "stream:80", "url to connect to stream")
}

func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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

		repoRequest := &stream.RepoRequest{
			Timestamp: time.Now().Format(time.RFC3339),
			Owner:     o,
			Repo:      re,
		}

		_, err = s.client.LogRepo(ctx, repoRequest)
		if err != nil {
			s.log.Error().Err(err).Msg("write to stream")
		}
	}
}
