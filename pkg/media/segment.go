package media

import (
	"context"
	"fmt"
	"io"
	"m3u8-downloader/pkg/constants"
	"m3u8-downloader/pkg/httpClient"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type SegmentJob struct {
	URI       string
	Seq       uint64
	VariantID int
	Variant   *StreamVariant
}

func (j SegmentJob) AbsoluteURL() string {
	rel, _ := url.Parse(j.URI)
	return j.Variant.BaseURL.ResolveReference(rel).String()
}

func (j SegmentJob) Key() string {
	return fmt.Sprintf("%d:%s", j.Seq, j.URI)
}

func DownloadSegment(ctx context.Context, client *http.Client, segmentURL string, outputDir string) error {
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		req, err := http.NewRequestWithContext(ctx, "GET", segmentURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", constants.HTTPUserAgent)
		req.Header.Set("Referer", constants.REFERRER)

		resp, err := client.Do(req)
		if err != nil {
			if attempt == 1 {
				return err
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			io.Copy(io.Discard, resp.Body)
			httpErr := &httpClient.HttpError{Code: resp.StatusCode}
			if resp.StatusCode == 403 && attempt == 0 {
				continue
			}
			return httpErr
		}

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		fileName := safeFileName(path.Join(outputDir, path.Base(segmentURL)))
		out, err := os.Create(fileName)
		if err != nil {
			return err
		}
		defer out.Close()

		n, err := io.Copy(out, resp.Body)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("zero-byte download for %s", segmentURL)
		}
		return nil
	}
	return fmt.Errorf("exhausted retries")
}

func safeFileName(base string) string {
	if i := strings.IndexAny(base, "?&#"); i >= 0 {
		base = base[:i]
	}
	if base == "" {
		base = fmt.Sprintf("seg-%d.ts", time.Now().UnixNano())
	}
	return base
}
