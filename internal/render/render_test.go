package render

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/Mineru98/irasutoya-cli/internal/irasutoya"
)

type fakePreviewer struct {
	err  error
	urls []string
}

func (p *fakePreviewer) ShowURL(imageURL string) error {
	p.urls = append(p.urls, imageURL)
	return p.err
}

func TestIrasutoWritesExpectedOutput(t *testing.T) {
	item := fixtureIrasuto()
	previewer := &fakePreviewer{}
	var out bytes.Buffer

	if err := Irasuto(&out, previewer, item); err != nil {
		t.Fatalf("Irasuto returned error: %v", err)
	}

	want := "" +
		"Page URL:    http://example.com\n" +
		"Title:       title\n" +
		"Description: description\n" +
		"Image URL:   http://example.com/test.png\n" +
		"Image URL:   http://example.com/test2.png\n"
	if got := out.String(); got != want {
		t.Fatalf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}

	wantURLs := []string{"http://example.com/test.png", "http://example.com/test2.png"}
	if !reflect.DeepEqual(previewer.urls, wantURLs) {
		t.Fatalf("preview URLs mismatch: want %v, got %v", wantURLs, previewer.urls)
	}
}

func TestIrasutoWarnsOnUnsupportedTerminal(t *testing.T) {
	item := fixtureIrasuto()
	previewer := &fakePreviewer{err: UnsupportedTerminalError{}}
	var out bytes.Buffer

	if err := Irasuto(&out, previewer, item); err != nil {
		t.Fatalf("Irasuto returned error: %v", err)
	}

	want := "" +
		"Page URL:    http://example.com\n" +
		"Title:       title\n" +
		"Description: description\n" +
		"Image URL:   http://example.com/test.png\n" +
		"Image URL:   http://example.com/test2.png\n" +
		"warn: This terminal is not able to show images inline\n" +
		"warn: Please use a supported terminal or enable the cross-platform image fallback.\n"
	if got := out.String(); got != want {
		t.Fatalf("output mismatch\nwant:\n%q\ngot:\n%q", want, got)
	}

	if wantCalls := 1; len(previewer.urls) != wantCalls {
		t.Fatalf("unsupported preview should stop after first failed URL: want %d calls, got %d", wantCalls, len(previewer.urls))
	}
}

func TestIrasutoReturnsPreviewError(t *testing.T) {
	previewErr := errors.New("preview failed")
	var out bytes.Buffer

	err := Irasuto(&out, &fakePreviewer{err: previewErr}, fixtureIrasuto())
	if !errors.Is(err, previewErr) {
		t.Fatalf("expected preview error %v, got %v", previewErr, err)
	}
}

func fixtureIrasuto() irasutoya.Irasuto {
	return irasutoya.Irasuto{
		URL:         "http://example.com",
		Title:       "title",
		Description: "description",
		ImageURLs: []string{
			"http://example.com/test.png",
			"http://example.com/test2.png",
		},
	}
}
