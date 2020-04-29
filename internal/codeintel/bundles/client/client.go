package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/neelance/parallel"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

var bundleManagerURL = env.Get("PRECISE_CODE_INTEL_BUNDLE_MANAGER_URL", "", "precise-code-intel-bundle-manager URL")

var requestMeter = metrics.NewRequestMeter("precise_code_intel_bundle_manager", "Total number of requests sent to precise-code-intel-bundel-manager.")

var defaultTransport = &ot.Transport{
	RoundTripper: requestMeter.Transport(&http.Transport{
		MaxIdleConnsPerHost: 500,
	}, func(u *url.URL) string {
		return u.Path
	}),
}

var DefaultClient = newClient(bundleManagerURL, &http.Client{
	Transport: defaultTransport,
})

// Client is the interface to the precise-code-intel-bundle-manager service.
type Client interface {
	// BundleClient creates a client that can answer intelligence queries for a single dump.
	BundleClient(bundleID int) BundleClient

	// SendUpload transfers a raw LSIF upload to the bundle manager to be stored on disk.
	SendUpload(ctx context.Context, bundleID int, r io.Reader) error

	// GetUploads retrieves a raw LSIF upload from disk. The file is written to a file in the
	// given directory with a random filename. The generated filename is returned on success.
	GetUpload(ctx context.Context, bundleID int, dir string) (string, error)

	// SendDB transfers a converted database to the bundle manager to be stored on disk.
	SendDB(ctx context.Context, bundleID int, r io.Reader) error
}

type clientImpl struct {
	url         string
	httpClient  httpcli.Doer
	httpLimiter *parallel.Run
	userAgent   string
}

var _ Client = &clientImpl{}

func newClient(url string, httpClient httpcli.Doer) *clientImpl {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &clientImpl{
		url:         url,
		httpClient:  httpClient,
		httpLimiter: parallel.NewRun(500),
		userAgent:   filepath.Base(os.Args[0]),
	}
}

// BundleClient creates a client that can answer intelligence queries for a single dump.
func (c *clientImpl) BundleClient(bundleID int) BundleClient {
	return &bundleClientImpl{
		client:   c,
		bundleID: bundleID,
	}
}

// SendUpload transfers a raw LSIF upload to the bundle manager to be stored on disk.
func (c *clientImpl) SendUpload(ctx context.Context, bundleID int, r io.Reader) (err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "client.SendUpload")
	span.SetTag("bundleID", bundleID)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
		}
		span.Finish()
	}()

	url, err := makeURL(c.url, fmt.Sprintf("uploads/%d", bundleID), nil)
	if err != nil {
		return err
	}

	body, err := c.do(ctx, span, "POST", url, r)
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// GetUploads retrieves a raw LSIF upload from disk. The file is written to a file in the
// given directory with a random filename. The generated filename is returned on success.
func (c *clientImpl) GetUpload(ctx context.Context, bundleID int, dir string) (_ string, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "client.GetUpload")
	span.SetTag("bundleID", bundleID)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
		}
		span.Finish()
	}()

	url, err := makeURL(c.url, fmt.Sprintf("uploads/%d", bundleID), nil)
	if err != nil {
		return "", err
	}

	body, err := c.do(ctx, span, "GET", url, nil)
	if err != nil {
		return "", err
	}
	defer body.Close()

	f, err := openRandomFile(dir)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()

	if _, err := io.Copy(f, body); err != nil {
		return "", err
	}

	return f.Name(), nil
}

// SendDB transfers a converted database to the bundle manager to be stored on disk.
func (c *clientImpl) SendDB(ctx context.Context, bundleID int, r io.Reader) (err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "client.SendDB")
	span.SetTag("bundleID", bundleID)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
		}
		span.Finish()
	}()

	url, err := makeURL(c.url, fmt.Sprintf("dbs/%d", bundleID), nil)
	if err != nil {
		return err
	}

	body, err := c.do(ctx, span, "POST", url, r)
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

func (c *clientImpl) queryBundle(ctx context.Context, bundleID int, op string, qs map[string]interface{}, target interface{}) (err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "client.queryBundle")
	span.SetTag("op", op)
	span.SetTag("bundleID", bundleID)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
		}
		span.Finish()
	}()

	url, err := makeBundleURL(c.url, bundleID, op, qs)
	if err != nil {
		return err
	}

	body, err := c.do(ctx, span, "GET", url, nil)
	if err != nil {
		return err
	}
	defer body.Close()

	return json.NewDecoder(body).Decode(&target)
}

func (c *clientImpl) do(ctx context.Context, span opentracing.Span, method string, url *url.URL, body io.Reader) (_ io.ReadCloser, err error) {
	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req = req.WithContext(ctx)

	if c.httpLimiter != nil {
		c.httpLimiter.Acquire()
		defer c.httpLimiter.Release()
		span.LogKV("event", "Acquired HTTP limiter")
	}

	req, ht := nethttp.TraceRequest(
		span.Tracer(),
		req,
		nethttp.OperationName("BundleManager Client"),
		nethttp.ClientTrace(false),
	)
	defer ht.Finish()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func openRandomFile(dir string) (*os.File, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	return os.Create(filepath.Join(dir, uuid.String()))
}

func makeURL(baseURL, path string, qs map[string]interface{}) (*url.URL, error) {
	values := url.Values{}
	for k, v := range qs {
		values[k] = []string{fmt.Sprintf("%v", v)}
	}

	url, err := url.Parse(fmt.Sprintf("%s/%s", baseURL, path))
	if err != nil {
		return nil, err
	}
	url.RawQuery = values.Encode()
	return url, nil
}

func makeBundleURL(baseURL string, bundleID int, op string, qs map[string]interface{}) (*url.URL, error) {
	return makeURL(baseURL, fmt.Sprintf("dbs/%d/%s", bundleID, op), qs)
}
