// Package timescar talks to the Times Car Share website's station search,
// reusing the cookies captured by `login`.
package timescar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/yz4230/timescarshare-cli/internal/session"
)

const baseURL = "https://share.timescar.jp"

// Mode selects which search box to use.
type Mode string

const (
	// ModeAddress searches by address (住所).
	ModeAddress Mode = "address"
	// ModeStation searches by station name (ステーション名).
	ModeStation Mode = "station"
)

// ParseMode validates a --by value.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case ModeAddress, ModeStation:
		return Mode(s), nil
	default:
		return "", fmt.Errorf("unknown search mode %q (want address or station)", s)
	}
}

// Station is one search hit.
type Station struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Result is one page of search results.
type Result struct {
	Query     string    `json:"query"`
	Mode      Mode      `json:"mode"`
	Page      int       `json:"page"`
	PageCount int       `json:"page_count"`
	Total     int       `json:"total"`
	Stations  []Station `json:"stations"`
}

// Client is a logged-in Times Car Share HTTP client.
type Client struct {
	http *http.Client
}

// New builds a client that authenticates with the given session cookies.
func New(s *session.Session) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	byDomain := map[string][]*http.Cookie{}
	for _, c := range s.Cookies {
		byDomain[c.Domain] = append(byDomain[c.Domain], &http.Cookie{
			Name: c.Name, Value: c.Value, Path: c.Path, Secure: c.Secure, HttpOnly: c.HTTPOnly,
		})
	}
	for domain, cookies := range byDomain {
		u := &url.URL{Scheme: "https", Host: strings.TrimPrefix(domain, ".")}
		jar.SetCookies(u, cookies)
	}
	return &Client{http: &http.Client{Jar: jar}}, nil
}

var errNotLoggedIn = errors.New("not logged in (session expired); run `timescarshare-cli login` again")

// Search runs a station search and returns the requested page of results.
// Page numbering is 1-based; the search is always submitted first so that page
// navigation works even for a fresh session.
func (c *Client) Search(ctx context.Context, query string, mode Mode, page int) (*Result, error) {
	if page < 1 {
		page = 1
	}

	form := url.Values{}
	form.Set("l:loginForm/view/station/search.html", "l:loginForm")
	kbn := "1"
	if mode == ModeStation {
		kbn = "2"
	}
	form.Set("l:loginForm:localSearchKbn", kbn)
	form.Set("l:loginForm:nameAdr-s", query)
	form.Set("l:loginForm:doNameAdrSearch.x", "31")
	form.Set("l:loginForm:doNameAdrSearch.y", "11")

	body, err := c.post(ctx, baseURL+"/view/station/search.jsp", form)
	if err != nil {
		return nil, err
	}
	if isLoginPage(body) {
		return nil, errNotLoggedIn
	}

	if page > 1 {
		body, err = c.get(ctx, fmt.Sprintf("%s/view/station/list.jsp?pageNo=%d", baseURL, page))
		if err != nil {
			return nil, err
		}
		if isLoginPage(body) {
			return nil, errNotLoggedIn
		}
	}

	res, err := parseResult(body)
	if err != nil {
		return nil, err
	}
	res.Query = query
	res.Mode = mode
	res.Page = page
	return res, nil
}

func (c *Client) post(ctx context.Context, url string, form url.Values) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", baseURL+"/view/station/search.jsp")
	req.Header.Set("Origin", baseURL)
	return c.do(req)
}

func (c *Client) get(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Referer", baseURL+"/view/station/list.jsp")
	return c.do(req)
}

func (c *Client) do(req *http.Request) (string, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s %s: %s", req.Method, req.URL, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func isLoginPage(body string) bool {
	return strings.Contains(body, `id="tpLoginForm"`)
}

func parseResult(body string) (*Result, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse results: %w", err)
	}
	res := &Result{}
	if v, ok := doc.Find("#naviResultCount-hidden").First().Attr("value"); ok {
		res.Total, _ = strconv.Atoi(strings.TrimSpace(v))
	}
	if v, ok := doc.Find("#naviPageCount-hidden").First().Attr("value"); ok {
		res.PageCount, _ = strconv.Atoi(strings.TrimSpace(v))
	}
	for _, row := range doc.Find("tr").EachIter() {
		name := strings.TrimSpace(row.Find("#stationNm").First().Text())
		if name == "" {
			continue
		}
		st := Station{
			Name:    name,
			Address: strings.TrimSpace(row.Find("#adr1").First().Text()),
		}
		if href, ok := row.Find("a#goDetail").First().Attr("href"); ok {
			if u, err := url.Parse(href); err == nil {
				st.Code = u.Query().Get("scd")
			}
		}
		res.Stations = append(res.Stations, st)
	}
	return res, nil
}
