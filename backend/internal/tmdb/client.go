package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var (
	ErrNotConfigured  = errors.New("TMDB credentials are not configured")
	ErrQueryRequired  = errors.New("movie search query is required")
	ErrInvalidMovieID = errors.New("TMDB movie ID is invalid")
)

type Movie struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	PosterURL   string  `json:"poster_url"`
	ReleaseDate string  `json:"release_date"`
	VoteAverage float64 `json:"vote_average"`
	Runtime     int     `json:"runtime,omitempty"`
}

// GetMovie returns fields such as runtime that are not included in search results.
func (c *Client) GetMovie(ctx context.Context, movieID int64) (*Movie, error) {
	if movieID < 1 {
		return nil, ErrInvalidMovieID
	}
	if c.accessToken == "" && c.apiKey == "" {
		return nil, ErrNotConfigured
	}

	endpoint, _ := url.Parse(fmt.Sprintf("https://api.themoviedb.org/3/movie/%d", movieID))
	values := endpoint.Query()
	values.Set("language", "th-TH")
	if c.apiKey != "" {
		values.Set("api_key", c.apiKey)
	}
	endpoint.RawQuery = values.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create TMDB movie request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if c.accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("get TMDB movie: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB movie details returned status %d", response.StatusCode)
	}

	var result struct {
		ID          int64   `json:"id"`
		Title       string  `json:"title"`
		Overview    string  `json:"overview"`
		PosterPath  string  `json:"poster_path"`
		ReleaseDate string  `json:"release_date"`
		VoteAverage float64 `json:"vote_average"`
		Runtime     int     `json:"runtime"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode TMDB movie response: %w", err)
	}

	posterURL := ""
	if result.PosterPath != "" && c.imageBaseURL != "" {
		posterURL = c.imageBaseURL + "/" + strings.TrimLeft(result.PosterPath, "/")
	}

	return &Movie{
		ID: result.ID, Title: result.Title, Overview: result.Overview,
		PosterURL: posterURL, ReleaseDate: result.ReleaseDate,
		VoteAverage: result.VoteAverage, Runtime: result.Runtime,
	}, nil
}

type Client struct {
	httpClient   *http.Client
	accessToken  string
	apiKey       string
	imageBaseURL string
}

func NewClient(
	httpClient *http.Client,
	accessToken string,
	apiKey string,
	imageBaseURL string,
) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	accessToken = strings.TrimSpace(accessToken)
	apiKey = strings.TrimSpace(apiKey)

	// TMDB Read Access Tokens are JWT-like and are sometimes copied into the
	// API key variable. Treat long JWT values as bearer tokens automatically.
	if accessToken == "" && (strings.Count(apiKey, ".") == 2 || len(apiKey) > 100) {
		accessToken = strings.TrimPrefix(apiKey, "Bearer ")
		apiKey = ""
	}

	return &Client{
		httpClient:   httpClient,
		accessToken:  accessToken,
		apiKey:       apiKey,
		imageBaseURL: strings.TrimRight(imageBaseURL, "/"),
	}
}

// SearchMovies searches TMDB on the server so credentials never reach the browser.
func (c *Client) SearchMovies(ctx context.Context, query string) ([]Movie, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrQueryRequired
	}
	if c.accessToken == "" && c.apiKey == "" {
		return nil, ErrNotConfigured
	}

	endpoint, _ := url.Parse("https://api.themoviedb.org/3/search/movie")
	values := endpoint.Query()
	values.Set("query", query)
	values.Set("language", "th-TH")
	values.Set("include_adult", "false")
	if c.apiKey != "" {
		values.Set("api_key", c.apiKey)
	}
	endpoint.RawQuery = values.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create TMDB search request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if c.accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("search TMDB movies: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB search returned status %d", response.StatusCode)
	}

	var payload struct {
		Results []struct {
			ID          int64   `json:"id"`
			Title       string  `json:"title"`
			Overview    string  `json:"overview"`
			PosterPath  string  `json:"poster_path"`
			ReleaseDate string  `json:"release_date"`
			VoteAverage float64 `json:"vote_average"`
		} `json:"results"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode TMDB search response: %w", err)
	}

	movies := make([]Movie, 0, len(payload.Results))
	for _, result := range payload.Results {
		posterURL := ""
		if result.PosterPath != "" && c.imageBaseURL != "" {
			posterURL = c.imageBaseURL + "/" + strings.TrimLeft(result.PosterPath, "/")
		}
		movies = append(movies, Movie{
			ID: result.ID, Title: result.Title, Overview: result.Overview,
			PosterURL: posterURL, ReleaseDate: result.ReleaseDate,
			VoteAverage: result.VoteAverage,
		})
	}

	return movies, nil
}
