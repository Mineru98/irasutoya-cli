package irasutoya

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	baseURL        = "https://www.irasutoya.com"
	randomMaxIndex = 22208
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	httpClient HTTPClient
	random     *rand.Rand
}

func NewClient(httpClient HTTPClient) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{httpClient: httpClient, random: rand.New(rand.NewSource(rand.Int63()))}
}

func (c *Client) Random(ctx context.Context) (Irasuto, error) {
	randomURL, err := c.randomURL(ctx)
	if err != nil {
		return Irasuto{}, err
	}
	return c.FetchIrasuto(ctx, randomURL)
}

func (c *Client) Search(ctx context.Context, query string, page int) ([]IrasutoLink, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s", baseURL, url.QueryEscape(query))
	if page > 0 {
		searchURL = fmt.Sprintf("%s/search?q=%s&max-results=20&start=%d&by-date=false", baseURL, url.QueryEscape(query), page*20)
	}
	doc, err := c.fetchDocument(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	return ParseListPage(doc), nil
}

func (c *Client) FetchIrasuto(ctx context.Context, showURL string) (Irasuto, error) {
	doc, err := c.fetchDocument(ctx, showURL)
	if err != nil {
		return Irasuto{}, err
	}
	parsed := ParseShowPage(doc)
	parsed.URL = showURL
	return parsed, nil
}

func (c *Client) randomURL(ctx context.Context) (string, error) {
	index := c.random.Intn(randomMaxIndex)
	apiURL := fmt.Sprintf("%s/feeds/posts/summary?start-index=%d&max-results=1&alt=json-in-script", baseURL, index)
	body, err := c.fetchBody(ctx, apiURL)
	if err != nil {
		return "", err
	}

	start := strings.IndexByte(body, '{')
	end := strings.LastIndexByte(body, '}')
	if start < 0 || end < start {
		return "", errors.New("random response did not contain JSON payload")
	}

	var payload struct {
		Feed struct {
			Entry []struct {
				Link []struct {
					Rel  string `json:"rel"`
					Href string `json:"href"`
				} `json:"link"`
			} `json:"entry"`
		} `json:"feed"`
	}
	if err := json.Unmarshal([]byte(body[start:end+1]), &payload); err != nil {
		return "", err
	}
	if len(payload.Feed.Entry) == 0 {
		return "", errors.New("random response did not contain entries")
	}
	for _, link := range payload.Feed.Entry[0].Link {
		if link.Rel == "alternate" && link.Href != "" {
			return link.Href, nil
		}
	}
	return "", errors.New("random response did not contain alternate link")
}

func (c *Client) fetchDocument(ctx context.Context, pageURL string) (*goquery.Document, error) {
	body, err := c.fetchBody(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(strings.NewReader(body))
}

func (c *Client) fetchBody(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s: unexpected status %s", pageURL, resp.Status)
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func ParseListPage(doc *goquery.Document) []IrasutoLink {
	var links []IrasutoLink
	doc.Find(".box").Each(func(_ int, box *goquery.Selection) {
		anchors := box.Find("a")
		showURL, exists := anchors.First().Attr("href")
		if !exists {
			return
		}
		title := strings.TrimSpace(anchors.Eq(1).Text())
		links = append(links, IrasutoLink{Title: title, ShowURL: showURL})
	})
	return links
}

func ParseShowPage(doc *goquery.Document) Irasuto {
	return Irasuto{
		Title:       strings.TrimSpace(doc.Find(".post .title").Find("h2").Text()),
		Description: showDescription(doc),
		ImageURLs:   showImageURLs(doc),
	}
}

func showDescription(doc *goquery.Document) string {
	separators := doc.Find(".entry .separator")
	target := separators.First()
	if separators.Length() > 1 {
		target = separators.Eq(1)
	}
	return strings.TrimSpace(target.Text())
}

func showImageURLs(doc *goquery.Document) []string {
	var imageURLs []string
	doc.Find(".entry").Find("img").Each(func(_ int, image *goquery.Selection) {
		src, exists := image.Attr("src")
		if !exists || src == "" {
			return
		}
		if strings.HasPrefix(src, "/") {
			src = "https:" + src
		}
		imageURLs = append(imageURLs, src)
	})
	return imageURLs
}
