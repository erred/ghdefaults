package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/go-github/v31/github"
	"github.com/rs/zerolog"

	"github.com/bradleyfalzon/ghinstallation"
)

var (
	AppID int64 = 62448

	port = func() string {
		port := os.Getenv("PORT")
		if port == "" {
			port = ":8080"
		} else if port[0] != ':' {
			port = ":" + port
		}
		return port
	}()
)

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		cancel()
	}()

	// server
	NewServer(os.Args).Run(ctx)
}

type Server struct {
	WebHookSecret string

	// server
	log zerolog.Logger
	mux *http.ServeMux
	srv *http.Server
}

func NewServer(args []string) *Server {
	s := &Server{
		log: zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, NoColor: true, TimeFormat: time.RFC3339}).With().Timestamp().Logger(),
		mux: http.NewServeMux(),
		srv: &http.Server{
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}

	s.mux.Handle("/", http.RedirectHandler("https://github.com/seankhliao/github-defaults", http.StatusFound))
	s.mux.Handle("/webhook", s)

	s.srv.Handler = s.mux
	s.srv.ErrorLog = log.New(s.log, "", 0)

	var priv string
	fs := flag.NewFlagSet(args[0], flag.ExitOnError)
	fs.StringVar(&s.srv.Addr, "addr", port, "host:port to serve on")
	flag.StringVar(&s.WebHookSecret, "webhook-secret", os.Getenv("WEBHOOK_SECRET"), "webhook validation secret")
	flag.StringVar(&priv, "priv", os.Getenv("PRIVATE_KEY"), "github app private key literal")
	fs.Parse(args[1:])

	return s
}

func (s *Server) Run(ctx context.Context) {
	errc := make(chan error)
	go func() {
		errc <- s.srv.ListenAndServe()
	}()

	var err error
	select {
	case err = <-errc:
	case <-ctx.Done():
		err = s.srv.Shutdown(ctx)
	}
	s.log.Error().Err(err).Msg("server exit")
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(s.WebHookSecret))
	if err != nil {
		s.log.Debug().Err(err).Msg("webhook validation failed")
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		s.log.Debug().Err(err).Msg("webhook parse failed")
		return
	}

	switch event := event.(type) {
	case *github.RepositoryEvent:
		if *event.Action != "created" {
			break
		}
		repo := &github.Repository{
			HasIssues:        github.Bool(false),
			HasWiki:          github.Bool(false),
			HasProjects:      github.Bool(false),
			AllowMergeCommit: github.Bool(false),
			AllowRebaseMerge: github.Bool(false),
		}

		itr, err := ghinstallation.NewKeyFromFile(
			http.DefaultTransport, AppID, *event.Installation.ID, "ghkey.pem")
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
