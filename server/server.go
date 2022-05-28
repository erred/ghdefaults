package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/go-logr/logr"
	"github.com/google/go-github/v41/github"
	"go.seankhliao.com/svcrunner"
	"go.seankhliao.com/svcrunner/envflag"
	"golang.org/x/oauth2"
)

var defaultConfig = map[string]github.Repository{
	"erred": {
		AllowMergeCommit:    github.Bool(false),
		AllowSquashMerge:    github.Bool(true),
		AllowRebaseMerge:    github.Bool(false),
		DeleteBranchOnMerge: github.Bool(true),
		HasIssues:           github.Bool(false),
		HasWiki:             github.Bool(false),
		HasPages:            github.Bool(false),
		HasProjects:         github.Bool(false),
		IsTemplate:          github.Bool(false),
		Archived:            github.Bool(true),
	},
	"seankhliao": {
		AllowMergeCommit:    github.Bool(false),
		AllowSquashMerge:    github.Bool(true),
		AllowRebaseMerge:    github.Bool(false),
		DeleteBranchOnMerge: github.Bool(true),
		HasIssues:           github.Bool(false),
		HasWiki:             github.Bool(false),
		HasPages:            github.Bool(false),
		HasProjects:         github.Bool(false),
		IsTemplate:          github.Bool(false),
	},
}

type Server struct {
	appKeyFile     string
	hookSecretFile string
	appID          int64
	appKey         []byte
	hookSecret     []byte

	log logr.Logger
}

func New(hs *http.Server) *Server {
	s := &Server{}
	return s
}

func (s *Server) Register(c *envflag.Config) {
	c.Int64Var(&s.appID, "ghdefaults.app-id", 0, "github app id")

	c.StringVar(&s.appKeyFile, "gh.app-key-file", "/etc/mono/ghdefaults/github.pem", "file with github aoo key")
	c.StringVar(&s.hookSecretFile, "gh.webhook-secret-file", "/etc/mono/ghdefaults/WEBHOOK_SECRET", "file with shared webhook secret")
}

func (s *Server) Init(ctx context.Context, t svcrunner.Tools) error {
	s.log = t.Log.WithName("ghdefaults")

	var err error
	s.appKey, err = os.ReadFile(s.appKeyFile)
	if err != nil {
		return fmt.Errorf("read app key file %s: %w", s.appKeyFile, err)
	}
	s.appKey = bytes.TrimSpace(s.appKey)
	s.hookSecret, err = os.ReadFile(s.hookSecretFile)
	if err != nil {
		return fmt.Errorf("read shared webhook file %s: %w", s.hookSecretFile, err)
	}
	s.hookSecret = bytes.TrimSpace(s.hookSecret)
	return nil
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	event, eventType, err := s.getPayload(ctx, r)
	if err != nil {
		http.Error(rw, "invalid payload", http.StatusBadRequest)
		s.log.Error(err, "invalid payload", "user_agent", r.UserAgent())
		return
	}

	switch event := event.(type) {
	case *github.InstallationEvent:
		s.installEvent(ctx, event)
	case *github.RepositoryEvent:
		s.repoEvent(ctx, rw, event)
	default:
		s.log.Info("ignoring event", "event", eventType)
	}
}

func (s *Server) getPayload(ctx context.Context, r *http.Request) (any, string, error) {
	payload, err := github.ValidatePayload(r, s.hookSecret)
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

func (s *Server) installEvent(ctx context.Context, event *github.InstallationEvent) {
	owner := *event.Installation.Account.Login
	installID := *event.Installation.ID
	switch *event.Action {
	case "created":
		if _, ok := defaultConfig[owner]; !ok {
			return
		}

		s.log.Info("setting defaults", "repos", len(event.Repositories))
		go func() {
			for _, repo := range event.Repositories {
				err := s.setDefaults(ctx, installID, owner, *repo.Name)
				if err != nil {
					s.log.Error(err, "setting default", "repo", owner+"/"+*repo.Name)
					continue
				}
				s.log.Info("defaults set", "repo", owner+"/"+*repo.Name)
			}
		}()
	default:
		s.log.Info("ignoring install action", "action", *event.Action)
	}
}

func (s *Server) repoEvent(ctx context.Context, rw http.ResponseWriter, event *github.RepositoryEvent) {
	installID := *event.Installation.ID
	owner := *event.Repo.Owner.Login
	repo := *event.Repo.Name
	switch *event.Action {
	case "created", "transferred":
		if _, ok := defaultConfig[owner]; !ok {
			return
		}
		err := s.setDefaults(ctx, installID, owner, repo)
		if err != nil {
			s.log.Error(err, "setting defaults", "repo", owner+"/"+repo)
			http.Error(rw, "set defaults", http.StatusInternalServerError)
			return
		}
		s.log.Info("defaults set", "repo", owner+"/"+repo)
	default:
		s.log.Info("ignoring repo action", "action", *event.Action)
	}
}

func (s *Server) setDefaults(ctx context.Context, installID int64, owner, repo string) error {
	config := defaultConfig[owner]
	tr := http.DefaultTransport
	tr, err := ghinstallation.NewAppsTransport(tr, s.appID, s.appKey)
	if err != nil {
		return fmt.Errorf("create ghinstallation transport: %w", err)
	}
	client := github.NewClient(&http.Client{Transport: tr})
	installToken, _, err := client.Apps.CreateInstallationToken(ctx, installID, nil)
	if err != nil {
		return fmt.Errorf("create installation token: %w", err)
	}

	client = github.NewClient(&http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *installToken.Token}),
		},
	})

	_, _, err = client.Repositories.Edit(ctx, owner, repo, &config)
	if err != nil {
		return fmt.Errorf("update repo settings: %w", err)
	}
	return nil
}
