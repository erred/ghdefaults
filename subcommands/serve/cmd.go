package serve

import (
	"context"
	"flag"
	"os"

	"github.com/google/subcommands"
	"go.seankhliao.com/svcrunner/v2/tshttp"
)

type Cmd struct {
	tshttp tshttp.Config

	webhookSecret string
	privateKey    string
	appID         int64
}

func (c *Cmd) Name() string     { return `serve` }
func (c *Cmd) Synopsis() string { return `start server` }
func (c *Cmd) Usage() string {
	return `serve [options...]

Starts a server managing listening records

Flags:
`
}

func (c *Cmd) SetFlags(f *flag.FlagSet) {
	c.tshttp.SetFlags(f)

	f.StringVar(&c.webhookSecret, "gh.webhook.secret", "", "webhook shared secret")
	f.Int64Var(&c.appID, "gh.app.id", 0, "github app id")
	f.Func("gh.private.key", "private key path", func(s string) error {
		b, err := os.ReadFile(s)
		if err != nil {
			return err
		}
		c.privateKey = string(b)
		return nil
	})
}

func (c *Cmd) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	err := New(ctx, c).Run(ctx)
	if err != nil {
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}
