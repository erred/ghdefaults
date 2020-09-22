package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
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
	name        = "ghdefaults"
	AppID int64 = 62448
)

func main() {
	var s Server

	usvc.Run(context.Background(), name, &s, false)
}

type Server struct {
	WebHookSecret string
	privBase64    string
	pkey          []byte

	log zerolog.Logger

	streamAddr string
	client     stream.StreamClient
	cc         *grpc.ClientConn
}

func (s *Server) Flag(fs *flag.FlagSet) {
	fs.StringVar(&s.WebHookSecret, "webhook-secret", os.Getenv("WEBHOOK_SECRET"), "webhook validation secret")
	fs.StringVar(&s.privBase64, "priv", os.Getenv("PRIVATE_KEY"), "base64 encoded private key")
	fs.StringVar(&s.streamAddr, "stream.addr", "stream:80", "url to connect to stream")
}

func (s *Server) Register(c *usvc.Components) error {
	s.log = c.Log

	var err error
	s.pkey, err = base64.StdEncoding.DecodeString(s.privBase64)
	if err != nil {
		s.log.Error().Err(err).Msg("decode private key")
	}

	c.HTTP.Handle("/", s)

	s.cc, err = grpc.Dial(s.streamAddr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("connect to stream: %w", err)
	}
	s.client = stream.NewStreamClient(s.cc)
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.cc.Close()
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
