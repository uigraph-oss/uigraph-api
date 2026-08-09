package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// onlyReader hides any io.Seeker/io.WriterTo the underlying reader might implement, so the test
// actually exercises the "truly unknown size, non-seekable" path that broke in production --
// e.g. the response body from Download(), which maps.uploadScreenshot streams directly into
// Upload when copying a gateway-uploads/ temp object to its canonical asset key.
type onlyReader struct{ r io.Reader }

func (o *onlyReader) Read(p []byte) (int, error) { return o.r.Read(p) }

func TestS3Upload_UnknownSize(t *testing.T) {
	var gotContentLength string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentLength = r.Header.Get("Content-Length")
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := newS3Client(Config{
		Backend:        "s3",
		Endpoint:       srv.URL,
		Bucket:         "test-bucket",
		Region:         "us-east-1",
		AccessKey:      "test",
		SecretKey:      "test",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("newS3Client: %v", err)
	}

	const content = "hello from an unknown-length stream"
	err = client.Upload(context.Background(), "some/key", "text/plain", &onlyReader{r: strings.NewReader(content)}, -1)
	if err != nil {
		// This is exactly the production failure mode: AWS SDK v2's PutObject rejects a
		// non-seekable body with no Content-Length -- S3 itself returns
		// "MissingContentLength: You must provide the Content-Length HTTP header."
		t.Fatalf("Upload with size=-1 failed (this is the regression this test guards against): %v", err)
	}
	if gotContentLength == "" {
		t.Error("expected a Content-Length header on the outgoing PutObject request, got none")
	}
	if string(gotBody) != content {
		t.Errorf("uploaded body = %q, want %q", gotBody, content)
	}
}

func TestS3Upload_KnownSize(t *testing.T) {
	var gotContentLength string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentLength = r.Header.Get("Content-Length")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := newS3Client(Config{
		Backend:        "s3",
		Endpoint:       srv.URL,
		Bucket:         "test-bucket",
		Region:         "us-east-1",
		AccessKey:      "test",
		SecretKey:      "test",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("newS3Client: %v", err)
	}

	const content = "known-length content"
	err = client.Upload(context.Background(), "some/key", "text/plain", strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload with known size failed: %v", err)
	}
	if gotContentLength != "20" {
		t.Errorf("Content-Length header = %q, want %q", gotContentLength, "20")
	}
}
