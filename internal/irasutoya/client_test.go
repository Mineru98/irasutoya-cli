package irasutoya

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestParseListPageMatchesExpectedSelectors(t *testing.T) {
	doc := newDocument(t, `
		<html><body>
			<div class="box">
				<a href="https://example.com/first.html"><img src="first.png"></a>
				<a>first title</a>
			</div>
			<div class="box">
				<a href="https://example.com/second.html"><img src="second.png"></a>
				<a>second title</a>
			</div>
		</body></html>
	`)

	links := ParseListPage(doc)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0] != (IrasutoLink{Title: "first title", ShowURL: "https://example.com/first.html"}) {
		t.Fatalf("unexpected first link: %#v", links[0])
	}
	if links[1] != (IrasutoLink{Title: "second title", ShowURL: "https://example.com/second.html"}) {
		t.Fatalf("unexpected second link: %#v", links[1])
	}
}

func TestParseListPageSkipsBoxesWithoutShowURL(t *testing.T) {
	doc := newDocument(t, `
		<html><body>
			<div class="box"><a>missing href</a><a>missing</a></div>
			<div class="box"><a href="https://example.com/only.html"></a><a>only title</a></div>
		</body></html>
	`)

	links := ParseListPage(doc)
	if len(links) != 1 {
		t.Fatalf("expected 1 valid link, got %d: %#v", len(links), links)
	}
	if links[0] != (IrasutoLink{Title: "only title", ShowURL: "https://example.com/only.html"}) {
		t.Fatalf("unexpected link: %#v", links[0])
	}
}

func TestParseShowPageMatchesExpectedSelectors(t *testing.T) {
	doc := newDocument(t, `
		<html><body>
			<div class="post"><div class="title"><h2> title </h2></div></div>
			<div class="entry">
				<div class="separator">thumbnail</div>
				<div class="separator"> description </div>
				<img src="//blogger.googleusercontent.com/img/a.png">
				<img src="/img/b.png">
			</div>
		</body></html>
	`)

	got := ParseShowPage(doc)
	if got.Title != "title" {
		t.Fatalf("title mismatch: %q", got.Title)
	}
	if got.Description != "description" {
		t.Fatalf("description mismatch: %q", got.Description)
	}
	wantImages := []string{"https://blogger.googleusercontent.com/img/a.png", "https:/img/b.png"}
	if strings.Join(got.ImageURLs, "\n") != strings.Join(wantImages, "\n") {
		t.Fatalf("image URLs mismatch: want %#v, got %#v", wantImages, got.ImageURLs)
	}
}

func TestParseShowPageUsesFirstSeparatorWhenSecondIsMissing(t *testing.T) {
	doc := newDocument(t, `
		<html><body>
			<div class="post"><div class="title"><h2>fallback</h2></div></div>
			<div class="entry">
				<div class="separator"> description fallback </div>
				<img src="//example.com/fallback.png">
			</div>
		</body></html>
	`)

	got := ParseShowPage(doc)
	if got.Description != "description fallback" {
		t.Fatalf("description fallback mismatch: %q", got.Description)
	}
	if len(got.ImageURLs) != 1 || got.ImageURLs[0] != "https://example.com/fallback.png" {
		t.Fatalf("image URLs mismatch: %#v", got.ImageURLs)
	}
}

func TestClientSearchBuildsExpectedURLs(t *testing.T) {
	var requested []string
	client := NewClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		return htmlResponse(`<div class="box"><a href="https://example.com/show.html"></a><a>title</a></div>`), nil
	}))

	links, err := client.Search(context.Background(), "猫 犬", 1)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	wantURL := "https://www.irasutoya.com/search?q=%E7%8C%AB+%E7%8A%AC&max-results=20&start=20&by-date=false"
	if requested[0] != wantURL {
		t.Fatalf("search URL mismatch: want %q, got %q", wantURL, requested[0])
	}
}

func TestClientFetchIrasutoSetsShowURL(t *testing.T) {
	client := NewClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return htmlResponse(`
			<div class="post"><div class="title"><h2>title</h2></div></div>
			<div class="entry"><div class="separator">description</div><img src="//example.com/a.png"></div>
		`), nil
	}))

	got, err := client.FetchIrasuto(context.Background(), "https://example.com/show.html")
	if err != nil {
		t.Fatalf("FetchIrasuto returned error: %v", err)
	}
	if got.URL != "https://example.com/show.html" {
		t.Fatalf("URL mismatch: %q", got.URL)
	}
	if got.Title != "title" || got.Description != "description" || len(got.ImageURLs) != 1 {
		t.Fatalf("unexpected irasuto: %#v", got)
	}
}

func TestClientRandomUsesAlternateLink(t *testing.T) {
	client := NewClient(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/feeds/posts/summary") {
			return htmlResponse(`callback({"feed":{"entry":[{"link":[{"rel":"self","href":"self"},{"rel":"alternate","href":"https://example.com/random.html"}]}]}});`), nil
		}
		return htmlResponse(`
			<div class="post"><div class="title"><h2>random</h2></div></div>
			<div class="entry"><div class="separator">desc</div><img src="//example.com/random.png"></div>
		`), nil
	}))

	got, err := client.Random(context.Background())
	if err != nil {
		t.Fatalf("Random returned error: %v", err)
	}
	if got.URL != "https://example.com/random.html" || got.Title != "random" {
		t.Fatalf("unexpected random irasuto: %#v", got)
	}
}

func newDocument(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	return doc
}

func htmlResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
