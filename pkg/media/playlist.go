package media

import (
	"fmt"
	"github.com/grafov/m3u8"
	"m3u8-downloader/pkg/constants"
	"net/http"
)

func LoadMediaPlaylist(mediaURL string) (*m3u8.MediaPlaylist, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", mediaURL, nil)
	req.Header.Set("User-Agent", constants.HTTPUserAgent)
	req.Header.Set("Referer", constants.REFERRER)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	pl, listType, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return nil, err
	}
	if listType == m3u8.MASTER {
		return nil, fmt.Errorf("expected media playlist but got master")
	}
	return pl.(*m3u8.MediaPlaylist), nil
}
