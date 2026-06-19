package main

import (
	"context"
	"os"

	"github.com/Mineru98/irasutoya-cli/internal/cli"
	"github.com/Mineru98/irasutoya-cli/internal/irasutoya"
	"github.com/Mineru98/irasutoya-cli/internal/preview"
	"github.com/Mineru98/irasutoya-cli/internal/render"
)

func main() {
	root := cli.NewRootCommand(cli.Dependencies{
		Service:           irasutoya.NewClient(nil),
		Previewer:         render.NoopPreviewer{},
		ExternalPreviewer: preview.NewOpener(nil),
	})
	if err := root.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}
