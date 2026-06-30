// Package screenshot runs an in-process worker that regenerates diagram preview
// thumbnails by rendering the SPA's headless screenshot route via go-rod.
//
// Docker images ship Chromium via UIGRAPH_CHROMIUM_PATH. When unset, go-rod
// downloads and caches a browser binary on first use.
//
// Auth: the worker mints a short-lived service-account token (scope diagrams:read)
// per job and injects it as the X-API-Key header on the headless browser, so no
// static render key needs to be provisioned or configured.
package screenshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/url"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/cache"
	diagrampkg "github.com/uigraph/app/internal/diagram"
	"github.com/uigraph/app/internal/identity"
	"github.com/uigraph/app/internal/queue"
	"github.com/uigraph/app/internal/storage"
)

const (
	renderTimeout = 30 * time.Second
	pollInterval  = 1 * time.Second
	viewportW     = 1000
	viewportH     = 1000
	tokenTTL      = 5 * time.Minute
)

type store interface {
	GetDiagram(ctx context.Context, id string) (*diagrampkg.Diagram, error)
	UpdateDiagram(ctx context.Context, d diagrampkg.Diagram) error
}

type objectStore interface {
	Upload(ctx context.Context, key, contentType string, r io.Reader, size int64) error
}

// saStore lets the worker mint its own short-lived render credential in-process
// under the org's built-in System Service account, so no static service-account
// API key has to be provisioned and configured.
type saStore interface {
	GetSystemServiceAccount(ctx context.Context, orgID string) (*identity.ServiceAccount, error)
	CreateServiceAccount(ctx context.Context, sa identity.ServiceAccount) error
	CreateToken(ctx context.Context, t identity.Token) error
	RevokeToken(ctx context.Context, tokenID string) error
}

// Worker consumes screenshot jobs and writes diagram preview assets.
type Worker struct {
	queue        *queue.Queue
	store        store
	sa           saStore
	storage      objectStore
	cache        cache.Client
	frontendURL  string
	chromiumPath string

	mu      sync.Mutex
	saCache map[string]string
}

// New constructs a Worker. frontendURL is the SPA base (e.g. http://localhost:3000).
// chromiumPath may be empty to let go-rod download and cache the browser binary on
// first use. The worker mints its own diagrams:read render token per job.
func New(q *queue.Queue, s store, sa saStore, st objectStore, c cache.Client, frontendURL, chromiumPath string) *Worker {
	return &Worker{queue: q, store: s, sa: sa, storage: st, cache: c, frontendURL: frontendURL, chromiumPath: chromiumPath, saCache: map[string]string{}}
}

// Run consumes jobs until ctx is cancelled. It owns a single browser launched
// once and reused across jobs.
func (w *Worker) Run(ctx context.Context) {
	bin := w.chromiumPath
	if bin == "" {
		p, err := launcher.NewBrowser().Get()
		if err != nil {
			slog.ErrorContext(ctx, "screenshot worker: browser download failed", "err", err)
			return
		}
		bin = p
	}

	l := launcher.New().Bin(bin).Headless(true).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("hide-scrollbars").
		Set("window-size", fmt.Sprintf("%d,%d", viewportW, viewportH))
	controlURL, err := l.Launch()
	if err != nil {
		slog.ErrorContext(ctx, "screenshot worker: browser launch failed", "err", err)
		return
	}
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		slog.ErrorContext(ctx, "screenshot worker: browser connect failed", "err", err)
		return
	}
	defer browser.Close()

	slog.InfoContext(ctx, "screenshot worker started")
	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "screenshot worker stopping")
			return
		case <-time.After(pollInterval):
		}

		job, ok, err := w.queue.DequeueScreenshot(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.WarnContext(ctx, "screenshot dequeue failed", "err", err)
			continue
		}
		if !ok {
			continue
		}
		if err := w.process(browser, job); err != nil {
			slog.WarnContext(ctx, "screenshot job failed", "diagramId", job.DiagramID, "err", err)
		}
	}
}

func (w *Worker) process(browser *rod.Browser, job queue.ScreenshotJob) error {
	png, err := w.capture(browser, job)
	if err != nil {
		return err
	}

	bgCtx := context.Background()
	assetID := storage.DiagramThumbnailAssetID(job.DiagramID)
	if err := w.storage.Upload(bgCtx, storage.AssetKey(assetID), "image/png", bytes.NewReader(png), int64(len(png))); err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	dg, err := w.store.GetDiagram(bgCtx, job.DiagramID)
	if err != nil {
		return fmt.Errorf("get diagram: %w", err)
	}
	if dg == nil || dg.DeletedAt != nil {
		return nil
	}

	hash := sha256Hex(png)
	dg.PreviewAssetID = &assetID
	dg.PreviewContentHash = &hash
	if err := w.store.UpdateDiagram(bgCtx, *dg); err != nil {
		return fmt.Errorf("update diagram: %w", err)
	}

	if w.cache != nil {
		_ = w.cache.Del(bgCtx, cache.AssetURLKey(assetID))
	}
	slog.InfoContext(bgCtx, "diagram thumbnail regenerated", "diagramId", job.DiagramID)
	return nil
}

func (w *Worker) capture(browser *rod.Browser, job queue.ScreenshotJob) ([]byte, error) {
	tokenID, apiKey, err := w.mintToken(context.Background(), job.OrgID)
	if err != nil {
		return nil, fmt.Errorf("mint token: %w", err)
	}
	defer func() { _ = w.sa.RevokeToken(context.Background(), tokenID) }()

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, fmt.Errorf("new page: %w", err)
	}
	defer page.Close()
	page = page.Timeout(renderTimeout)

	if _, err := page.SetExtraHeaders([]string{"X-API-Key", apiKey}); err != nil {
		return nil, fmt.Errorf("set headers: %w", err)
	}

	target := fmt.Sprintf("%s/diagram-screenshot/%s?orgId=%s",
		w.frontendURL, url.PathEscape(job.DiagramID), url.QueryEscape(job.OrgID))
	if err := page.Navigate(target); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	if err := page.Wait(rod.Eval("() => window.__screenshotReady === true")); err != nil {
		return nil, fmt.Errorf("wait ready: %w", err)
	}

	clip, err := page.Eval("() => window.__screenshotClip")
	if err != nil {
		return nil, fmt.Errorf("read screenshot clip: %w", err)
	}
	c := clip.Value
	x, y := c.Get("x").Num(), c.Get("y").Num()
	cw, ch := c.Get("width").Num(), c.Get("height").Num()

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             int(math.Ceil(x + cw)),
		Height:            int(math.Ceil(y + ch)),
		DeviceScaleFactor: 2,
	}); err != nil {
		return nil, fmt.Errorf("set viewport: %w", err)
	}

	png, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
		Clip: &proto.PageViewport{
			X:      x,
			Y:      y,
			Width:  cw,
			Height: ch,
			Scale:  1,
		},
		CaptureBeyondViewport: true,
	})
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	return png, nil
}

func (w *Worker) mintToken(ctx context.Context, orgID string) (tokenID, plaintext string, err error) {
	saID, err := w.ensureSA(ctx, orgID)
	if err != nil {
		return "", "", err
	}
	id, plaintext, hash, err := identity.Generate()
	if err != nil {
		return "", "", err
	}
	exp := time.Now().UTC().Add(tokenTTL)
	tok := identity.Token{
		ID:               id,
		ServiceAccountID: saID,
		Name:             "screenshot-render-" + id,
		Prefix:           identity.Prefix(plaintext),
		Hash:             hash,
		ExpiresAt:        &exp,
	}
	if err := w.sa.CreateToken(ctx, tok); err != nil {
		return "", "", err
	}
	return id, plaintext, nil
}

func (w *Worker) ensureSA(ctx context.Context, orgID string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if id, ok := w.saCache[orgID]; ok {
		return id, nil
	}

	existing, err := w.sa.GetSystemServiceAccount(ctx, orgID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		w.saCache[orgID] = existing.ID
		return existing.ID, nil
	}

	sa := identity.NewSystemServiceAccount(orgID, systemScopes())
	if err := w.sa.CreateServiceAccount(ctx, sa); err != nil {
		return "", err
	}
	w.saCache[orgID] = sa.ID
	return sa.ID, nil
}

func systemScopes() []string {
	scopes := make([]string, len(authz.AllScopes))
	for i, s := range authz.AllScopes {
		scopes[i] = string(s)
	}
	return scopes
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
