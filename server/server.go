package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/go-logr/logr"
	"github.com/google/go-github/v45/github"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.seankhliao.com/svcrunner"
	"go.seankhliao.com/svcrunner/envflag"
	"golang.org/x/oauth2"
)

var ErrSetDefaults = errors.New("errors setting repo defaults")

var defaultConfig = map[string]github.Repository{
	"erred": {
		AllowMergeCommit:    github.Bool(false),
		AllowUpdateBranch:   github.Bool(true),
		AllowAutoMerge:      github.Bool(true),
		AllowSquashMerge:    github.Bool(true),
		AllowRebaseMerge:    github.Bool(false),
		DeleteBranchOnMerge: github.Bool(true),
		HasIssues:           github.Bool(false),
		HasWiki:             github.Bool(false),
		HasPages:            github.Bool(false),
		HasProjects:         github.Bool(false),
		HasDownloads:        github.Bool(false),
		IsTemplate:          github.Bool(false),
		Archived:            github.Bool(true),
	},
	"seankhliao": {
		AllowMergeCommit:    github.Bool(false),
		AllowUpdateBranch:   github.Bool(true),
		AllowAutoMerge:      github.Bool(true),
		AllowSquashMerge:    github.Bool(true),
		AllowRebaseMerge:    github.Bool(false),
		DeleteBranchOnMerge: github.Bool(true),
		HasIssues:           github.Bool(false),
		HasWiki:             github.Bool(false),
		HasPages:            github.Bool(false),
		HasProjects:         github.Bool(false),
		HasDownloads:        github.Bool(false),
		IsTemplate:          github.Bool(false),
	},
}

type Server struct {
	clientSecret  string // not used?
	webhookSecret string
	privateKey    string
	appID         int64

	log   logr.Logger
	trace trace.Tracer
}

func New(hs *http.Server) *Server {
	s := &Server{}
	mux := http.NewServeMux()
	mux.Handle("/webhook", s)
	hs.Handler = mux
	return s
}

func (s *Server) Register(c *envflag.Config) {
	c.Int64Var(&s.appID, "ghdefaults.app-id", 0, "github app id")
	c.StringVar(&s.clientSecret, "ghdefaults.client-secret", "", "client secret")
	c.StringVar(&s.webhookSecret, "ghdefaults.webhook-secret", "", "webhook shared secret")
	c.StringVar(&s.privateKey, "ghdefaults.private-key", "", "private key")
}

func (s *Server) Init(ctx context.Context, t svcrunner.Tools) error {
	s.log = t.Log.WithName("ghdefaults")
	s.trace = otel.Tracer("ghdefaults")

	s.privateKey += "\n"
	return nil
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx, span := s.trace.Start(r.Context(), "dispatch-event-type")
	defer span.End()

	event, eventType, err := s.getPayload(ctx, r)
	if err != nil {
		http.Error(rw, "invalid payload", http.StatusBadRequest)
		s.log.Error(err, "invalid payload", "ctx", ctx, "http_request", r)
		return
	}

	log := s.log.WithValues("event_type", eventType)

	switch event := event.(type) {
	case *github.InstallationEvent:
		err = s.installEvent(ctx, log, event)
	case *github.RepositoryEvent:
		err = s.repoEvent(ctx, log, event)
	default:
		log.V(1).Info("ignoring event", "event", eventType)
	}

	// error should already be logged
	if err != nil {
		http.Error(rw, "error setting defaults", http.StatusInternalServerError)
		return
	}
	rw.Write([]byte("ok"))
}

func (s *Server) getPayload(ctx context.Context, r *http.Request) (any, string, error) {
	ctx, span := s.trace.Start(ctx, "extract-payload")
	defer span.End()

	payload, err := github.ValidatePayload(r, []byte(s.webhookSecret))
	if err != nil {
		return nil, "", fmt.Errorf("validate: %w", err)
	}
	eventType := github.WebHookType(r)
	event, err := github.ParseWebHook(eventType, payload)
	if err != nil {
		return nil, "", fmt.Errorf("parse: %w", err)
	}
	return event, eventType, nil
}

func (s *Server) installEvent(ctx context.Context, log logr.Logger, event *github.InstallationEvent) error {
	ctx, span := s.trace.Start(ctx, "install-event")
	defer span.End()

	owner := *event.Installation.Account.Login
	installID := *event.Installation.ID
	log = log.WithValues("action", *event.Action)
	var errs error
	switch *event.Action {
	case "created":
		if _, ok := defaultConfig[owner]; !ok {
			log.V(1).Info("ignoring unknown owner")
			return nil
		}

		log.V(1).Info("setting defaults", "repos", len(event.Repositories), "ctx", ctx)
		for _, repo := range event.Repositories {
			log := s.log.WithValues("repo", owner+"/"+*repo.Name)
			err := s.setDefaults(ctx, installID, owner, *repo.Name)
			if err != nil {
				errs = ErrSetDefaults
				log.Error(err, "setting defaults", "ctx", ctx)
				continue
			}
		}
	default:
		log.V(1).Info("ignoring action", "ctx", ctx)
		return nil
	}

	if errs != nil {
		return errs
	}
	log.Info("all defaults set", "ctx", ctx)
	return nil
}

func (s *Server) repoEvent(ctx context.Context, log logr.Logger, event *github.RepositoryEvent) error {
	ctx, span := s.trace.Start(ctx, "repo-event")
	defer span.End()

	installID := *event.Installation.ID
	owner := *event.Repo.Owner.Login
	repo := *event.Repo.Name
	log = log.WithValues("action", *event.Action, "repo", owner+"/"+repo)
	switch *event.Action {
	case "created", "transferred":
		if _, ok := defaultConfig[owner]; !ok {
			log.V(1).Info("ignoring unknown owner", "ctx", ctx)
			return nil
		}
		err := s.setDefaults(ctx, installID, owner, repo)
		if err != nil {
			log.Error(err, "setting defaults", "ctx", ctx)
			return ErrSetDefaults
		}
	default:
		log.V(1).Info("ignoring action", "ctx", ctx)
		return nil
	}
	log.Info("defaults set", "ctx", ctx)
	return nil
}

func (s *Server) setDefaults(ctx context.Context, installID int64, owner, repo string) error {
	ctx, span := s.trace.Start(ctx, "set-defaults")
	defer span.End()

	config := defaultConfig[owner]
	tr := http.DefaultTransport
	tr, err := ghinstallation.NewAppsTransport(tr, s.appID, []byte(s.privateKey))
	if err != nil {
		return fmt.Errorf("create ghinstallation transport: %w", err)
	}
	client := github.NewClient(&http.Client{Transport: otelhttp.NewTransport(tr)})
	installToken, _, err := client.Apps.CreateInstallationToken(ctx, installID, nil)
	if err != nil {
		return fmt.Errorf("create installation token: %w", err)
	}

	client = github.NewClient(&http.Client{
		Transport: otelhttp.NewTransport(&oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *installToken.Token}),
		}),
	})

	_, _, err = client.Repositories.Edit(ctx, owner, repo, &config)
	if err != nil {
		return fmt.Errorf("update repo settings: %w", err)
	}
	return nil
}
