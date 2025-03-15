package auth

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

var CredentialErr = errors.New("fatal")

const expirationLayout = time.RFC3339

type (
	CredentialProcessOutput struct {
		Version         int
		AccessKeyId     string
		SecretAccessKey string
		SessionToken    string
		Expiration      string
	}

	cacheRetrieverSaver struct {
		Logger   *log.Logger
		cacheDir string
	}

	cacheFile struct {
		expiration time.Time
		filePath   string
	}

	cacheFileSlice []*cacheFile
)

func (cs cacheFileSlice) Len() int {
	return len(cs)
}

func (cs cacheFileSlice) Less(i, j int) bool {
	return cs[i].expiration.Before(cs[j].expiration)
}

func (cs cacheFileSlice) Swap(i, j int) {
	cs[i], cs[j] = cs[j], cs[i]
}

func GetPrefix(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])[0:7]

}

func EncodeToFileName(roleArn string, ts time.Time) string {
	return fmt.Sprintf("%s-%s.json", GetPrefix(roleArn), strconv.FormatInt(ts.Unix(), 10))
}

func DecodeFromFileName(roleArn, fileName string) (time.Time, error) {
	var ts time.Time
	regex := regexp.MustCompile(fmt.Sprintf(`^%s-(\d+)\.json$`, GetPrefix(roleArn)))
	matches := regex.FindStringSubmatch(fileName)
	if matches == nil {
		return ts, fmt.Errorf("%q is not of the right cache file name format", fileName)
	}
	unixSec, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return ts, fmt.Errorf("numeric portion of %q is not a valid Unix second: %w", fileName, err)
	}
	ts = time.Unix(unixSec, 0)
	return ts, nil
}

func NewCacheRetrieverSaver(logger *log.Logger) (*cacheRetrieverSaver, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.New("could not locate user home directory")
	}
	cacheDir := filepath.Join(home, ".aws", "toolkit-cache")
	info, err := os.Stat(cacheDir)
	if err == nil && info.IsDir() {
		return &cacheRetrieverSaver{Logger: logger, cacheDir: cacheDir}, nil
	}
	if err == nil && !info.IsDir() {
		return nil, errors.New("cache directory is already a file")
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return nil, errors.New("cannot create cache directory")
		}
		return &cacheRetrieverSaver{Logger: logger, cacheDir: cacheDir}, nil
	}
	return nil, err
}

func (c *cacheRetrieverSaver) Save(roleArn string, output *CredentialProcessOutput) ([]byte, error) {
	ts, err := time.Parse(expirationLayout, output.Expiration)
	if err != nil {
		return nil, fmt.Errorf("CredentialProcessOutput.Expiration %q is not of the right format: %w: %w", output.Expiration, err, CredentialErr)
	}
	fileName := EncodeToFileName(roleArn, ts)
	filePath := filepath.Join(c.cacheDir, fileName)
	contents, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize CredentialProcessOutput: %w: %w", err, CredentialErr)
	}
	if err = os.WriteFile(filePath, contents, 0644); err != nil {
		return contents, fmt.Errorf("failed to save credentials to cache file: %w", err)
	}
	return contents, nil
}

func (c *cacheRetrieverSaver) Retrieve(roleArn string) ([]byte, error) {
	max := time.Now().Add(time.Minute * 10)
	actives := make(cacheFileSlice, 0)
	pattern := filepath.Join(c.cacheDir, fmt.Sprintf(`%s-*.json`, GetPrefix(roleArn)))
	cacheFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid file globbing pattern: %w", err)
	}
	for _, fullPath := range cacheFiles {
		if expiration, err := DecodeFromFileName(roleArn, filepath.Base(fullPath)); err != nil {
			if os.Remove(fullPath) != nil {
				c.Logger.Printf("Failed to delete invalid cache file %q.\n", fullPath)
			}
			continue
		} else if expiration.Before(max) {
			if os.Remove(fullPath) != nil {
				c.Logger.Printf("Failed to delete almost expired cache file %q.\n", fullPath)
			}
			continue
		} else {
			actives = append(actives, &cacheFile{expiration: expiration, filePath: fullPath})
		}
	}
	if len(actives) == 0 {
		return nil, nil
	}
	sort.Sort(actives)
	for _, item := range actives[1:] {
		if os.Remove(item.filePath) != nil {
			c.Logger.Printf("Failed to delete older cache file %q.\n", item.filePath)
		}
	}
	contents, err := os.ReadFile(actives[0].filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read active cache file %q: %w", actives[0].filePath, err)
	}
	return contents, nil
}
