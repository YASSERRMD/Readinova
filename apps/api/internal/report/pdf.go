package report

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// GeneratePDF renders the HTML report to a PDF using a headless Chromium instance.
// Returns an error if Chrome/Chromium is not available on the host.
func GeneratePDF(ctx context.Context, htmlContent string) ([]byte, error) {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx,
		chromedp.NoSandbox,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	)
	defer cancelAlloc()

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
