package convertx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotConfigured = errors.New("convertx: not configured")
	ErrUnsupported   = errors.New("convertx: no converter for target format")
	ErrAuth          = errors.New("convertx: authentication failed")
	ErrConvertFailed = errors.New("convertx: conversion failed")
	ErrTimeout       = errors.New("convertx: conversion timed out")
	errUnauthorized  = errors.New("convertx: session invalid")
)

type Source struct {
	Open func() (io.ReadCloser, error)
	Size int64
}

const pollInterval = 2 * time.Second

type Client struct {
	base     string
	email    string
	password string
	timeout  time.Duration
	pollWait time.Duration
	http     *http.Client

	mu    sync.Mutex
	token string
}

func New(base, email, password string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &Client{
		base:     strings.TrimSuffix(base, "/"),
		email:    email,
		password: password,
		timeout:  timeout,
		pollWait: pollInterval,
		http: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) Convert(ctx context.Context, src Source, inputName, target, converter string, dst io.Writer) (string, error) {
	target = normalizeOutputExt(strings.ToLower(strings.TrimSpace(target)))
	if target == "" {
		return "", ErrUnsupported
	}
	if !Supported(target) {
		return "", fmt.Errorf("%w: %q", ErrUnsupported, target)
	}
	if converter != "" {
		if _, ok := converterSpecs[converter]; !ok {
			return "", fmt.Errorf("%w: unknown converter %q", ErrUnsupported, converter)
		}
	}
	if src.Open == nil {
		return "", errors.New("convertx: source has no opener")
	}

	for attempt := 0; ; attempt++ {
		name, err := c.convertOnce(ctx, src, inputName, target, converter, dst)
		if err == nil {
			return name, nil
		}
		if errors.Is(err, errUnauthorized) && attempt == 0 {
			c.mu.Lock()
			c.token = ""
			c.mu.Unlock()
			continue
		}
		return "", err
	}
}

func (c *Client) convertOnce(ctx context.Context, src Source, inputName, target, converter string, dst io.Writer) (string, error) {
	token, err := c.ensureAuth(ctx)
	if err != nil {
		return "", err
	}

	job, err := c.createJob(ctx, token)
	if err != nil {
		return "", err
	}
	if err := c.upload(ctx, token, job, src, inputName); err != nil {
		return "", err
	}
	jobID, err := c.startConvert(ctx, token, job, inputName, target, converter)
	if err != nil {
		return "", err
	}
	row, err := c.waitResult(ctx, token, jobID)
	if err != nil {
		return "", err
	}
	if err := c.download(ctx, token, row.href, dst); err != nil {
		return "", err
	}

	name := filepath.Base(strings.SplitN(row.href, "?", 2)[0])
	if u, err := url.PathUnescape(name); err == nil {
		name = u
	}
	return name, nil
}

func (c *Client) ensureAuth(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" {
		return c.token, nil
	}

	token, err := c.login(ctx)
	if err == nil {
		c.token = token
		return token, nil
	}
	if !errors.Is(err, ErrAuth) && !errors.Is(err, errUnauthorized) {
		return "", err
	}

	regErr := c.register(ctx)
	if regErr != nil && !strings.Contains(regErr.Error(), "in use") {
		return "", fmt.Errorf("convertx: %w (registration: %v)", err, regErr)
	}

	token, err = c.login(ctx)
	if err != nil {
		return "", err
	}
	c.token = token
	return token, nil
}

func (c *Client) login(ctx context.Context) (string, error) {
	resp, err := c.postJSON(ctx, "/login", map[string]string{"email": c.email, "password": c.password})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return "", errUnauthorized
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%w (status %d)", ErrAuth, resp.StatusCode)
	}
	return c.cookie(resp, "auth")
}

func (c *Client) register(ctx context.Context) error {
	resp, err := c.postJSON(ctx, "/register", map[string]string{"email": c.email, "password": c.password})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<12))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w (status %d: %s)", ErrAuth, resp.StatusCode, summarize(body))
	}
	return nil
}

func summarize(body []byte) string {
	s := strings.Join(strings.Fields(string(body)), " ")
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	if s == "" {
		return "empty response"
	}
	return s
}

func (c *Client) createJob(ctx context.Context, token string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/", nil, -1, nil, map[string]string{"auth": token})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return "", errUnauthorized
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("convertx: create job: status %d", resp.StatusCode)
	}
	job, err := c.cookie(resp, "jobId")
	if err != nil {
		return "", fmt.Errorf("convertx: %w", err)
	}
	return job, nil
}

func (c *Client) upload(ctx context.Context, token, job string, src Source, name string) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	contentType := mw.FormDataContentType()
	length, err := multipartLen(mw.Boundary(), "file", name, src.Size)
	if err != nil {
		return fmt.Errorf("convertx: %w", err)
	}

	go func() {
		pw.CloseWithError(func() error {
			body, err := src.Open()
			if err != nil {
				return err
			}
			defer body.Close()

			fw, err := mw.CreateFormFile("file", name)
			if err != nil {
				return err
			}
			if _, err := io.Copy(fw, body); err != nil {
				return err
			}
			return mw.Close()
		}())
	}()
	defer pr.Close()

	resp, err := c.do(ctx, http.MethodPost, "/upload", pr, length,
		map[string]string{"Content-Type": contentType},
		map[string]string{"auth": token, "jobId": job})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("convertx: upload: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) startConvert(ctx context.Context, token, job, name, target, converter string) (string, error) {
	if converter == "" {
		converter = ConverterFor(name, target)
	}
	if converter == "" {
		return "", fmt.Errorf("%w: %q", ErrUnsupported, target)
	}

	form := url.Values{}
	form.Set("convert_to", target+","+converter)
	fileNames, _ := json.Marshal([]string{name})
	form.Set("file_names", string(fileNames))

	resp, err := c.do(ctx, http.MethodPost, "/convert", strings.NewReader(form.Encode()), -1,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
		map[string]string{"auth": token, "jobId": job})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return "", errUnauthorized
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("convertx: convert: status %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return job, nil
	}
	loc = strings.TrimSuffix(loc, "/")
	segs := strings.Split(loc, "/")
	if n := len(segs); n > 0 {
		return segs[n-1], nil
	}
	return job, nil
}

func (c *Client) waitResult(ctx context.Context, token, jobID string) (resultRow, error) {
	deadline := time.Now().Add(c.timeout)
	for {
		page, err := c.getResults(ctx, token, jobID)
		if err != nil {
			return resultRow{}, err
		}
		for _, row := range parseResults(page) {
			if row.href == "" {
				continue
			}
			status := strings.ToLower(row.status)
			if strings.Contains(status, "fail") ||
				strings.Contains(status, "not supported") ||
				strings.Contains(status, "error") {
				return resultRow{}, fmt.Errorf("%w: %s", ErrConvertFailed, row.status)
			}
			return row, nil
		}

		if time.Now().After(deadline) {
			return resultRow{}, ErrTimeout
		}
		select {
		case <-ctx.Done():
			return resultRow{}, ctx.Err()
		case <-time.After(c.pollWait):
		}
	}
}

func (c *Client) getResults(ctx context.Context, token, jobID string) ([]byte, error) {
	resp, err := c.do(ctx, http.MethodGet, "/results/"+jobID, nil, -1, nil, map[string]string{"auth": token})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, errUnauthorized
	}
	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("convertx: results: status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) download(ctx context.Context, token, href string, dst io.Writer) error {
	resp, err := c.do(ctx, http.MethodGet, href, nil, -1, nil, map[string]string{"auth": token})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		_, _ = io.Copy(io.Discard, resp.Body)
		return errUnauthorized
	}
	if resp.StatusCode >= 400 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("convertx: download: status %d", resp.StatusCode)
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("convertx: download: %w", err)
	}
	return nil
}

// multipartLen is the exact byte length of the body CreateFormFile produces for
// a file of the given size: measured from a real writer rather than assembled by
// hand, so it cannot drift from what the upload actually sends.
func multipartLen(boundary, field, filename string, size int64) (int64, error) {
	var n countWriter
	mw := multipart.NewWriter(&n)
	if err := mw.SetBoundary(boundary); err != nil {
		return 0, err
	}
	if _, err := mw.CreateFormFile(field, filename); err != nil {
		return 0, err
	}
	if err := mw.Close(); err != nil {
		return 0, err
	}
	return int64(n) + size, nil
}

type countWriter int64

func (c *countWriter) Write(p []byte) (int, error) {
	*c += countWriter(len(p))
	return len(p), nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload map[string]string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("convertx: %w", err)
	}
	return c.do(ctx, http.MethodPost, path, bytes.NewReader(body), -1,
		map[string]string{"Content-Type": "application/json"}, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, length int64, headers, cookies map[string]string) (*http.Response, error) {
	target := path
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = c.base + path
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("convertx: %w", err)
	}
	if length >= 0 {
		req.ContentLength = length
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if len(cookies) > 0 {
		parts := make([]string, 0, len(cookies))
		for k, v := range cookies {
			parts = append(parts, k+"="+v)
		}
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}
	return c.http.Do(req)
}

func (c *Client) cookie(resp *http.Response, name string) (string, error) {
	for _, ck := range resp.Cookies() {
		if ck.Name == name {
			return ck.Value, nil
		}
	}
	return "", fmt.Errorf("no %q cookie in response", name)
}

type resultRow struct {
	name   string
	status string
	href   string
}

var (
	rowRe    = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	nameRe   = regexp.MustCompile(`(?i)<td[^>]*safe[^>]*>(.*?)</td>`)
	statusRe = regexp.MustCompile(`(?i)<td\s+safe>(.*?)</td>`)
	hrefRe   = regexp.MustCompile(`(?i)href="([^"]*?/download/[^"]*)"`)
)

func parseResults(page []byte) []resultRow {
	var rows []resultRow
	for _, m := range rowRe.FindAllSubmatch(page, -1) {
		row := m[1]
		r := resultRow{
			name:   firstMatch(nameRe, row),
			status: firstMatch(statusRe, row),
			href:   firstMatch(hrefRe, row),
		}
		if r.name == "" && r.status == "" && r.href == "" {
			continue
		}
		rows = append(rows, r)
	}
	return rows
}

func firstMatch(re *regexp.Regexp, b []byte) string {
	m := re.FindSubmatch(b)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}
