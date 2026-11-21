package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"ortalyq/backend/internal/apperr"
)

// Giphy is a minimal server-side GIPHY client. The API key never reaches the browser.
type Giphy struct {
	key  string
	base string
	http *http.Client
}

func NewGiphy(key, base string) *Giphy {
	return &Giphy{key: key, base: base, http: &http.Client{Timeout: 8 * time.Second}}
}

func (g *Giphy) Enabled() bool { return g.key != "" }

var gifIDRe = regexp.MustCompile(`^[A-Za-z0-9]{1,64}$`)

type giphyImage struct {
	URL    string `json:"url"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

type giphyGif struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Images struct {
		FixedHeight      giphyImage `json:"fixed_height"`
		FixedHeightSmall giphyImage `json:"fixed_height_small"`
	} `json:"images"`
}

func (x giphyGif) toGif() Gif {
	w, _ := strconv.Atoi(x.Images.FixedHeight.Width)
	h, _ := strconv.Atoi(x.Images.FixedHeight.Height)
	prev := x.Images.FixedHeightSmall.URL
	if prev == "" {
		prev = x.Images.FixedHeight.URL
	}
	return Gif{ID: x.ID, Title: x.Title, URL: x.Images.FixedHeight.URL, PreviewURL: prev, Width: w, Height: h}
}

func (g *Giphy) get(ctx context.Context, path string, q url.Values, out any) error {
	if !g.Enabled() {
		return apperr.Unavailable("giphy_not_configured", "GIF недоступны: не настроен ключ GIPHY")
	}
	q.Set("api_key", g.key)
	q.Set("rating", "g")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.base+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := g.http.Do(req)
	if err != nil {
		return apperr.Unavailable("giphy_unavailable", "Сервис GIF временно недоступен")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return apperr.NotFound("GIF")
		}
		return apperr.Unavailable("giphy_unavailable", fmt.Sprintf("Сервис GIF ответил ошибкой (%d)", resp.StatusCode))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (g *Giphy) Search(ctx context.Context, query string, offset int) ([]Gif, error) {
	q := url.Values{"limit": {"24"}, "offset": {strconv.Itoa(offset)}}
	path := "/v1/gifs/trending"
	if query != "" {
		path = "/v1/gifs/search"
		q.Set("q", query)
	}
	var body struct {
		Data []giphyGif `json:"data"`
	}
	if err := g.get(ctx, path, q, &body); err != nil {
		return nil, err
	}
	out := make([]Gif, 0, len(body.Data))
	for _, d := range body.Data {
		out = append(out, d.toGif())
	}
	return out, nil
}

// ByID verifies a GIF id with GIPHY so clients cannot inject arbitrary URLs.
func (g *Giphy) ByID(ctx context.Context, id string) (*Gif, error) {
	if !gifIDRe.MatchString(id) {
		return nil, apperr.Validation(map[string]string{"gif_id": "Некорректный GIF"})
	}
	var body struct {
		Data giphyGif `json:"data"`
	}
	if err := g.get(ctx, "/v1/gifs/"+id, url.Values{}, &body); err != nil {
		return nil, err
	}
	if body.Data.ID == "" {
		return nil, apperr.NotFound("GIF")
	}
	gif := body.Data.toGif()
	return &gif, nil
}
