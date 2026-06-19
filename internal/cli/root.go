package cli

import (
	"context"
	"io"
	"os"

	"github.com/Mineru98/irasutoya-cli/internal/irasutoya"
	"github.com/Mineru98/irasutoya-cli/internal/render"
	"github.com/spf13/cobra"
)

type Service interface {
	Random(ctx context.Context) (irasutoya.Irasuto, error)
	Search(ctx context.Context, query string, page int) ([]irasutoya.IrasutoLink, error)
	FetchIrasuto(ctx context.Context, showURL string) (irasutoya.Irasuto, error)
}

type Dependencies struct {
	Service           Service
	Previewer         render.Previewer
	ExternalPreviewer render.Previewer
	Out               io.Writer
}

func NewRootCommand(deps Dependencies) *cobra.Command {
	openImages := os.Getenv("IRASUTOYA_OPEN_IMAGES") == "1"
	cmd := &cobra.Command{
		Use:           "irasutoya",
		Short:         "CLI tool for irasutoya",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	cmd.PersistentFlags().BoolVar(&openImages, "open-images", openImages, "open image URLs with the OS default application")
	if deps.Out != nil {
		cmd.SetOut(deps.Out)
		cmd.SetErr(deps.Out)
	}

	selectedPreviewer := func() render.Previewer {
		if openImages {
			return deps.ExternalPreviewer
		}
		return deps.Previewer
	}

	cmd.AddCommand(newRandomCommand(deps, selectedPreviewer))
	cmd.AddCommand(newSearchCommand(deps, selectedPreviewer))
	return cmd
}

func newRandomCommand(deps Dependencies, selectedPreviewer func() render.Previewer) *cobra.Command {
	return &cobra.Command{
		Use:   "random",
		Short: "Gives you random irasutoya image",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			item, err := deps.Service.Random(cmd.Context())
			if err != nil {
				return err
			}
			return render.Irasuto(cmd.OutOrStdout(), selectedPreviewer(), item)
		},
	}
}

func newSearchCommand(deps Dependencies, selectedPreviewer func() render.Previewer) *cobra.Command {
	return &cobra.Command{
		Use:   "search {query}",
		Short: "Gives you 3 irasutoya images by given query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			links, err := deps.Service.Search(cmd.Context(), normalizeSearchQuery(args[0]), 0)
			if err != nil {
				return err
			}
			if len(links) > 3 {
				links = links[:3]
			}
			for _, link := range links {
				item, err := deps.Service.FetchIrasuto(cmd.Context(), link.ShowURL)
				if err != nil {
					return err
				}
				if isEmptyIrasuto(item) {
					continue
				}
				if err := render.Irasuto(cmd.OutOrStdout(), selectedPreviewer(), item); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func isEmptyIrasuto(item irasutoya.Irasuto) bool {
	return item.URL == "" && item.Title == "" && item.Description == "" && len(item.ImageURLs) == 0
}

func normalizeSearchQuery(query string) string {
	switch query {
	case "고양이":
		return "猫"
	case "luffy", "Luffy", "LUFFY":
		return "ルフィ"
	case "루피":
		return "ルフィ"
	case "路飞":
		return "ルフィ"
	case "zoro", "Zoro", "ZORO":
		return "ゾロ"
	case "조로":
		return "ゾロ"
	case "索隆":
		return "ゾロ"
	case "원피스":
		return "ONE PIECE"
	default:
		return query
	}
}
