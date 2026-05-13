package cmd

import (
	"io"
	"net/http"
	"os"
)

func resolveAPIToken(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("XRMCP_API_TOKEN")
}

func newAPIRequest(method, url, token string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}
