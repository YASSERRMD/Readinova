package report

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// browserPool holds a single shared browser allocator context that is
// initialised once and reused across PDF requests, avoiding the overhead of
// launching a new Chromium process per request.
var (
	allocCtxOnce   sync.Once
	sharedAllocCtx context.Context
	sharedAllocErr error
)

// getOrCreateAllocator returns a shared browser allocator context.
// On first call it launches the headless Chromium process; subsequent calls
// reuse the existing one.
func getOrCreateAllocator() (context.Context, error) {
	allocCtxOnce.Do(func() {
		var cancel context.CancelFunc
		sharedAllocCtx, cancel = chromedp.NewExecAllocator(
			context.Background(),
			chromedp.NoSandbox,
			chromedp.Headless,
			chromedp.DisableGPU,
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
		)
		// cancel is intentionally not called here — the allocator lives for
		// the entire process lifetime.  A shutdown hook can call it if needed.
		_ = cancel
	})
	return sharedAllocCtx, sharedAllocErr
}

// GeneratePDF renders the HTML report to a PDF using a pooled headless Chromium
// instance.  Returns an error if Chrome/Chromium is not available on the host.
func GeneratePDF(ctx context.Context, htmlContent string) ([]byte, error) {
	allocCtx, err := getOrCreateAllocator()
	if err != nil {
		return nil, fmt.Errorf("chromedp allocator: %w", err)
	}

	// Each PDF gets its own browser tab context derived from the shared allocator.
	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	timeoutCtx, cancelTimeout := context.WithTimeout(taskCtx, 30*time.Second)
	defer cancelTimeout()

	dataURL := fmt.Sprintf("data:text/html;charset=utf-8,%s", urlEncodeHTML(htmlContent))

	var pdfBuf []byte
	if err := chromedp.Run(timeoutCtx,
		chromedp.Navigate(dataURL),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(8.5).
				WithPaperHeight(11).
				Do(ctx)
			return err
		}),
	); err != nil {
		return nil, fmt.Errorf("chromedp render: %w", err)
	}
	return pdfBuf, nil
}

// urlEncodeHTML percent-encodes characters that would break a data URL.
func urlEncodeHTML(s string) string {
	var out []byte
	for i := range len(s) {
		c := s[i]
		switch c {
		case ' ':
			out = append(out, '%', '2', '0')
		case '#':
			out = append(out, '%', '2', '3')
		case '%':
			out = append(out, '%', '2', '5')
		case '&':
			out = append(out, '%', '2', '6')
		case '+':
			out = append(out, '%', '2', 'B')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
