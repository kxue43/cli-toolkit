package scaffold

import (
	"fmt"
	"io"
	"net/http"

	"github.com/kxue43/cli-toolkit/jsonstream"
)

var pypiURL = "https://pypi.org/pypi"

func PackageLatestVersion(name string) (version string, err error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s/json", pypiURL, name))
	if err != nil {
		return version, fmt.Errorf("failed to get package %q data from PyPI: %w", name, err)
	}

	if rc := resp.StatusCode; rc != 200 {
		return "", fmt.Errorf("failed to get package %q data, status code %d", name, rc)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	angler, err := jsonstream.NewAngler(resp.Body, ".info.version")
	if err != nil {
		return "", fmt.Errorf("error from jsonstream.NewAngler: %w", err)
	}

	value, err := angler.Land()
	if err != nil {
		return "", fmt.Errorf(`failed to get the value at the ".info.version" path from the response body: %w`, err)
	}

	version, ok := value.(string)
	if !ok {
		return "", fmt.Errorf(`the value at the ".info.version" path is not string`)
	}

	return version, nil
}
