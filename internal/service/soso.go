package service

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func applySoSoHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://www.sosovalue.com")
	req.Header.Set("Referer", "https://www.sosovalue.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
}

func decodeUpstreamHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	message := strings.TrimSpace(string(body))
	if isCloudflareBlocked(resp.StatusCode, message) {
		return fmt.Errorf("upstream %s blocked by Cloudflare; server IP or request fingerprint is being challenged", resp.Status)
	}
	upstreamErr := fmt.Errorf("upstream %s: %s", resp.Status, message)
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
		return &retryableUpstreamError{err: upstreamErr}
	}
	return upstreamErr
}

func isCloudflareBlocked(statusCode int, body string) bool {
	if statusCode != http.StatusForbidden {
		return false
	}
	body = strings.ToLower(body)
	return strings.Contains(body, "cloudflare") || strings.Contains(body, "you have been blocked")
}
