//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

func getResponse(url string) ([]byte, error) {
	if len(url) == 0 {
		return nil, errors.New("invalid URL")
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	code := resp.StatusCode
	body, err := io.ReadAll(resp.Body)
	if err == nil && code != http.StatusOK {
		return nil, fmt.Errorf(string(body))
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("not ok: %v", http.StatusText(code))
	}
	return body, nil
}
