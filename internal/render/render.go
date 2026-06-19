package render

import (
	"fmt"
	"io"

	"github.com/Mineru98/irasutoya-cli/internal/irasutoya"
)

type Previewer interface {
	ShowURL(imageURL string) error
}

type NoopPreviewer struct{}

func (NoopPreviewer) ShowURL(string) error {
	return nil
}

type UnsupportedTerminalError struct{}

func (UnsupportedTerminalError) Error() string {
	return "terminal does not support inline images"
}

func Irasuto(w io.Writer, previewer Previewer, item irasutoya.Irasuto) error {
	if _, err := fmt.Fprintf(w, "Page URL:    %s\n", item.URL); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Title:       %s\n", item.Title); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Description: %s\n", item.Description); err != nil {
		return err
	}
	for _, imageURL := range item.ImageURLs {
		if _, err := fmt.Fprintf(w, "Image URL:   %s\n", imageURL); err != nil {
			return err
		}
	}

	if previewer == nil {
		return nil
	}
	for _, imageURL := range item.ImageURLs {
		if err := previewer.ShowURL(imageURL); err != nil {
			if _, ok := err.(UnsupportedTerminalError); ok {
				fmt.Fprintln(w, "warn: This terminal is not able to show images inline")
				fmt.Fprintln(w, "warn: Please use a supported terminal or enable the cross-platform image fallback.")
				return nil
			}
			return err
		}
	}
	return nil
}
