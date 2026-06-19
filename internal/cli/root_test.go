package cli

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Mineru98/irasutoya-cli/internal/irasutoya"
)

type fakeService struct {
	random       irasutoya.Irasuto
	searchLinks  []irasutoya.IrasutoLink
	fetched      map[string]irasutoya.Irasuto
	fetchErrors  map[string]error
	searchCalls  []string
	fetchedCalls []string
}

func (s *fakeService) Random(context.Context) (irasutoya.Irasuto, error) {
	return s.random, nil
}

func (s *fakeService) Search(_ context.Context, query string, page int) ([]irasutoya.IrasutoLink, error) {
	s.searchCalls = append(s.searchCalls, query)
	return s.searchLinks, nil
}

func (s *fakeService) FetchIrasuto(_ context.Context, showURL string) (irasutoya.Irasuto, error) {
	s.fetchedCalls = append(s.fetchedCalls, showURL)
	if err := s.fetchErrors[showURL]; err != nil {
		return irasutoya.Irasuto{}, err
	}
	item, ok := s.fetched[showURL]
	if !ok {
		return irasutoya.Irasuto{}, nil
	}
	return item, nil
}

type fakePreviewer struct {
	urls []string
}

func (p *fakePreviewer) ShowURL(imageURL string) error {
	p.urls = append(p.urls, imageURL)
	return nil
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

func TestRandomCommandDisplaysRandomIrasuto(t *testing.T) {
	t.Setenv("IRASUTOYA_OPEN_IMAGES", "")
	service := &fakeService{random: fixtureIrasuto("http://example.com/random")}
	previewer := &fakePreviewer{}
	out := runCommand(t, service, previewer, "random")

	want := "" +
		"Page URL:    http://example.com/random\n" +
		"Title:       title\n" +
		"Description: description\n" +
		"Image URL:   http://example.com/random/test.png\n" +
		"Image URL:   http://example.com/random/test2.png\n"
	if out != want {
		t.Fatalf("random output mismatch\nwant:\n%q\ngot:\n%q", want, out)
	}
	if len(previewer.urls) != 2 {
		t.Fatalf("expected 2 preview calls, got %d", len(previewer.urls))
	}
}

func TestOpenImagesFlagUsesExternalPreviewer(t *testing.T) {
	t.Setenv("IRASUTOYA_OPEN_IMAGES", "")
	service := &fakeService{random: fixtureIrasuto("http://example.com/random")}
	defaultPreviewer := &fakePreviewer{}
	externalPreviewer := &fakePreviewer{}
	var out bytes.Buffer
	cmd := NewRootCommand(Dependencies{
		Service:           service,
		Previewer:         defaultPreviewer,
		ExternalPreviewer: externalPreviewer,
		Out:               &out,
	})
	cmd.SetArgs([]string{"--open-images", "random"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if len(defaultPreviewer.urls) != 0 {
		t.Fatalf("default previewer should not be used with --open-images, got %v", defaultPreviewer.urls)
	}
	if len(externalPreviewer.urls) != 2 {
		t.Fatalf("expected 2 external preview calls, got %d", len(externalPreviewer.urls))
	}
}

func TestSearchCommandTakesFirstThreeLinksAndCompactsFetchFailures(t *testing.T) {
	t.Setenv("IRASUTOYA_OPEN_IMAGES", "")
	service := &fakeService{
		searchLinks: []irasutoya.IrasutoLink{
			{Title: "one", ShowURL: "https://example.com/one"},
			{Title: "two", ShowURL: "https://example.com/two"},
			{Title: "three", ShowURL: "https://example.com/three"},
			{Title: "four", ShowURL: "https://example.com/four"},
		},
		fetched: map[string]irasutoya.Irasuto{
			"https://example.com/one":   fixtureIrasuto("https://example.com/one"),
			"https://example.com/three": fixtureIrasuto("https://example.com/three"),
		},
	}
	previewer := &fakePreviewer{}

	out := runCommand(t, service, previewer, "search", "query")

	wantFetched := []string{"https://example.com/one", "https://example.com/two", "https://example.com/three"}
	if !reflect.DeepEqual(service.fetchedCalls, wantFetched) {
		t.Fatalf("fetched calls mismatch: want %v, got %v", wantFetched, service.fetchedCalls)
	}
	if strings.Contains(out, "https://example.com/four") {
		t.Fatalf("search should not fetch/display fourth result:\n%s", out)
	}
	if strings.Count(out, "Page URL:") != 2 {
		t.Fatalf("expected two displayed results after one fetch failure, got:\n%s", out)
	}
	want := "" +
		"Page URL:    https://example.com/one\n" +
		"Title:       title\n" +
		"Description: description\n" +
		"Image URL:   https://example.com/one/test.png\n" +
		"Image URL:   https://example.com/one/test2.png\n" +
		"Page URL:    https://example.com/three\n" +
		"Title:       title\n" +
		"Description: description\n" +
		"Image URL:   https://example.com/three/test.png\n" +
		"Image URL:   https://example.com/three/test2.png\n"
	if out != want {
		t.Fatalf("search output mismatch\nwant:\n%q\ngot:\n%q", want, out)
	}
	if len(previewer.urls) != 4 {
		t.Fatalf("expected previews for two displayed results with two images each, got %d", len(previewer.urls))
	}
}

func TestSearchCommandReturnsFetchErrors(t *testing.T) {
	t.Setenv("IRASUTOYA_OPEN_IMAGES", "")
	fetchErr := errors.New("fetch failed")
	service := &fakeService{
		searchLinks: []irasutoya.IrasutoLink{
			{Title: "one", ShowURL: "https://example.com/one"},
		},
		fetchErrors: map[string]error{"https://example.com/one": fetchErr},
	}
	var out bytes.Buffer
	cmd := NewRootCommand(Dependencies{Service: service, Previewer: &fakePreviewer{}, Out: &out})
	cmd.SetArgs([]string{"search", "query"})

	err := cmd.ExecuteContext(context.Background())
	if !errors.Is(err, fetchErr) {
		t.Fatalf("expected fetch error %v, got %v", fetchErr, err)
	}
	if !strings.Contains(out.String(), "fetch failed") {
		t.Fatalf("expected visible fetch error output, got %q", out.String())
	}
}

func TestSearchCommandNormalizesLocalizedOnePieceQueries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "english luffy", input: "luffy", want: "ルフィ"},
		{name: "english zoro", input: "zoro", want: "ゾロ"},
		{name: "korean luffy", input: "루피", want: "ルフィ"},
		{name: "korean zoro", input: "조로", want: "ゾロ"},
		{name: "chinese luffy", input: "路飞", want: "ルフィ"},
		{name: "chinese zoro", input: "索隆", want: "ゾロ"},
		{name: "korean one piece", input: "원피스", want: "ONE PIECE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("IRASUTOYA_OPEN_IMAGES", "")
			service := &fakeService{
				searchLinks: []irasutoya.IrasutoLink{
					{Title: "one", ShowURL: "https://example.com/one"},
				},
				fetched: map[string]irasutoya.Irasuto{
					"https://example.com/one": fixtureIrasuto("https://example.com/one"),
				},
			}

			runCommand(t, service, &fakePreviewer{}, "search", tc.input)

			want := []string{tc.want}
			if !reflect.DeepEqual(service.searchCalls, want) {
				t.Fatalf("search query mismatch: want %v, got %v", want, service.searchCalls)
			}
		})
	}
}

func TestHelpIncludesCommands(t *testing.T) {
	t.Setenv("IRASUTOYA_OPEN_IMAGES", "")
	out := runCommand(t, &fakeService{}, nil, "help")

	for _, want := range []string{"random", "search"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output missing %q:\n%s", want, out)
		}
	}
}

func runCommand(t *testing.T, service Service, previewer interface{ ShowURL(string) error }, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd := NewRootCommand(Dependencies{Service: service, Previewer: previewer, Out: &out})
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	return out.String()
}

func fixtureIrasuto(pageURL string) irasutoya.Irasuto {
	return irasutoya.Irasuto{
		URL:         pageURL,
		Title:       "title",
		Description: "description",
		ImageURLs: []string{
			pageURL + "/test.png",
			pageURL + "/test2.png",
		},
	}
}
