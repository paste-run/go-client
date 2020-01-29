package paste // import "paste.run"

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type request struct {
	author  string
	title   string
	desc    string
	typ     string
	tok     string
	ctx     context.Context
	client  *http.Client
	baseURL string
	headers []string
	query   string
}

// Option is one of the request options.
type Option func(*request)

// Author of the paste for upload.
func Author(set string) Option {
	return func(req *request) {
		req.author = set
	}
}

// Title of the paste for upload.
func Title(set string) Option {
	return func(req *request) {
		req.title = set
	}
}

// Description of the paste for upload.
func Description(set string) Option {
	return func(req *request) {
		req.desc = set
	}
}

// Type of paste for upload.
func Type(set string) Option {
	return func(req *request) {
		req.typ = set
	}
}

// Token for the request.
func Token(set string) Option {
	return func(req *request) {
		req.tok = set
	}
}

// Context for the request.
func Context(set context.Context) Option {
	return func(req *request) {
		req.ctx = set
	}
}

// Client for the request.
func Client(set *http.Client) Option {
	return func(req *request) {
		req.client = set
	}
}

const defaultBaseURL = "https://api.paste.run/"

// BaseURL - This function may be removed in the future.
func BaseURL(set string) Option {
	return func(req *request) {
		req.baseURL = set
	}
}

// Headers for the request.
// headers are HTTP header pairs of key, value.
func Headers(headers ...string) Option {
	if len(headers)%2 != 0 {
		panic("invalid headers")
	}
	return func(req *request) {
		req.headers = headers
	}
}

// Query is the query for GetLanguages.
func Query(set string) Option {
	return func(req *request) {
		req.query = set
	}
}

func upload(r io.Reader, req *request, options ...Option) (string, error) {
	for _, opt := range options {
		opt(req)
	}

	bodyr, bodyw := io.Pipe()
	defer bodyr.Close() // Don't hang writes if bailing out.
	w := multipart.NewWriter(bodyw)
	contentType := w.FormDataContentType()

	go func() {
		bodywClosed := false
		defer func() {
			err := w.Close() // Done with the multipart writer.
			if !bodywClosed {
				bodyw.CloseWithError(err)
				bodywClosed = true
			}
		}()

		if req.author != "" {
			w.WriteField("author", req.author)
		}
		if req.title != "" {
			w.WriteField("title", req.title)
		}
		if req.desc != "" {
			w.WriteField("desc", req.desc)
		}
		if req.typ != "" {
			w.WriteField("type", req.typ)
		}

		f, err := w.CreateFormFile("file", "-")
		if err != nil {
			bodyw.CloseWithError(err)
			bodywClosed = true
			return
		}
		_, err = io.Copy(f, r)
		if err != nil {
			bodyw.CloseWithError(err)
			bodywClosed = true
			return
		}
	}()

	url := req.baseURL
	if url == "" {
		url = defaultBaseURL
	}
	hr, err := http.NewRequest("POST", url, bodyr)
	if err != nil {
		return "", err
	}

	for i := 0; i+1 < len(req.headers); i += 2 {
		hr.Header.Set(req.headers[i], req.headers[i+1])
	}

	hr.Header.Set("Content-Type", contentType)

	if req.ctx != nil {
		hr = hr.WithContext(req.ctx)
	}

	if req.tok != "" {
		hr.Header.Set("Authorization", "Bearer "+req.tok)
	}

	client := req.client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(hr)
	if err != nil {
		return "", err
	}
	result, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 201 {
		return "", errors.New(strings.TrimSpace(string(result)))
	}
	return strings.TrimSpace(string(result)), nil
}

// Upload the paste in r. Returns the new paste URL.
func Upload(r io.Reader, options ...Option) (string, error) {
	return upload(r, &request{}, options...)
}

// UploadFile is a shortcut to Upload a file on the filesystem.
func UploadFile(path string, options ...Option) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fn := filepath.Base(path)
	return upload(f, &request{
		title: fn,
	}, options...)
}

func get(paste string, req *request, options ...Option) (PasteInfo, error) {
	for _, opt := range options {
		opt(req)
	}

	baseURL := req.baseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	pasteURL := ""
	if strings.Index(paste, "://") != -1 { // Paste URL.
		const p = "https://www.paste.run/"
		if !strings.HasPrefix(paste, p) || strings.ContainsAny(paste[len(p):], "./#?") {
			return PasteInfo{}, errors.New("invalid paste URL")
		}
		pasteURL = strings.TrimSuffix(baseURL, "/") + "/" + paste[len(p):] + "?raw"
	} else { // Paste ID.
		if strings.ContainsAny(paste, "./#?") {
			return PasteInfo{}, errors.New("invalid paste URL")
		}
		pasteURL = strings.TrimSuffix(baseURL, "/") + "/" + paste + "?raw"
	}

	hr, err := http.NewRequest("GET", pasteURL, nil)
	if err != nil {
		return PasteInfo{}, err
	}

	for i := 0; i+1 < len(req.headers); i += 2 {
		hr.Header.Set(req.headers[i], req.headers[i+1])
	}

	if req.ctx != nil {
		hr = hr.WithContext(req.ctx)
	}

	if req.tok != "" {
		hr.Header.Set("Authorization", "Bearer "+req.tok)
	}

	client := req.client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(hr)
	if err != nil {
		return PasteInfo{}, err
	}
	if resp.StatusCode != 200 {
		result, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return PasteInfo{}, err
		}
		return PasteInfo{}, errors.New(strings.TrimSpace(string(result)))
	}
	created, _ := time.Parse(http.TimeFormat, resp.Header.Get("Created-At"))
	expires, _ := time.Parse(http.TimeFormat, resp.Header.Get("Expires"))
	return PasteInfo{
		resp.Body,
		resp.ContentLength,
		resp.Header.Get("Content-Type"),
		resp.Header.Get("Paste-Language"),
		resp.Header.Get("Paste-Class"),
		resp.Header.Get("Created-By"),
		resp.Header.Get("Paste-Title"),
		created,
		expires,
	}, nil
}

// PasteInfo is information related to a paste.
// Content needs to be closed.
type PasteInfo struct {
	Content  io.ReadCloser `json:"-"`    // Paste content
	Size     int64         `json:"size"` // Size of the paste content in bytes
	Type     string        `json:"type"`
	Language string        `json:"language"`
	Class    string        `json:"class"` // Classifier: file name, .ext, mime type, etc
	Author   string        `json:"author"`
	Title    string        `json:"title"`
	Created  time.Time     `json:"created"`
	Expires  time.Time     `json:"expires"` // IsZero if no expiration
}

// Get a paste.
// paste can be a full paste URL or just the paste ID.
// The returned reader gets the raw content.
func Get(paste string, options ...Option) (PasteInfo, error) {
	return get(paste, &request{}, options...)
}

type LanguageInfo struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	Mode  string `json:"mode,omitempty"`
}

func getLanguages(req *request, options ...Option) ([]LanguageInfo, error) {
	for _, opt := range options {
		opt(req)
	}

	baseURL := req.baseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	geturl := strings.TrimSuffix(baseURL, "/") + "/languages"
	if req.query != "" {
		geturl += "?q=" + url.QueryEscape(req.query)
	}
	hr, err := http.NewRequest("GET", geturl, nil)
	if err != nil {
		return nil, err
	}

	hr.Header.Set("Accept", "application/json")

	for i := 0; i+1 < len(req.headers); i += 2 {
		hr.Header.Set(req.headers[i], req.headers[i+1])
	}

	if req.ctx != nil {
		hr = hr.WithContext(req.ctx)
	}

	if req.tok != "" {
		hr.Header.Set("Authorization", "Bearer "+req.tok)
	}

	client := req.client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(hr)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		result, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New(strings.TrimSpace(string(result)))
	}

	var x struct {
		Q       string         `json:"q,omitempty"`
		Results []LanguageInfo `json:"results"`
	}
	err = json.NewDecoder(resp.Body).Decode(&x)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return x.Results, nil
}

// GetLanguages gets information on all languages,
// or use Query(string) to search for particular language(s).
func GetLanguages(options ...Option) ([]LanguageInfo, error) {
	return getLanguages(&request{}, options...)
}
