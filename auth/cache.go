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

type (
	CredentialProcessOutput struct {
		AccessKeyId     string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
		SessionToken    string `json:"SessionToken"`
		Expiration      string `json:"Expiration"`
		Version         int    `json:"Version"`
	}

	cacheSaveRetriever struct {
		Logger   *log.Logger
		cacheDir string
	}

	cacheFile struct {
		expiration time.Time
		filePath   string
	}

	cacheFileSlice []*cacheFile
)

const expirationLayout = time.RFC3339

var ErrInvalidCredential = errors.New("invalid AWS credential")

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

func DecodeFromFileName(roleArn, fileName string) (ts time.Time, err error) {
	regex := regexp.MustCompile(fmt.Sprintf(`^%s-(\d+)\.json$`, GetPrefix(roleArn)))

	matches := regex.FindStringSubmatch(fileName)
	if matches == nil {
		return ts, fmt.Errorf("%q is not of the right cache file name format", fileName)
	}

	unixSec, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return ts, fmt.Errorf("numeric portion of %q is not a valid Unix second", fileName)
	}

	ts = time.Unix(unixSec, 0)

	return ts, nil
}

func NewCacheSaveRetriever(logger *log.Logger) (*cacheSaveRetriever, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.New("could not locate user home directory")
	}

	cacheDir := filepath.Join(home, ".aws", "toolkit-cache")

	info, err := os.Stat(cacheDir)

	if os.IsNotExist(err) {
		if err = os.MkdirAll(cacheDir, 0750); err != nil {
			return nil, errors.New("failed to create cache directory")
		}

		return &cacheSaveRetriever{Logger: logger, cacheDir: cacheDir}, nil
	} else if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, errors.New("cache directory is already a file")
	}

	return &cacheSaveRetriever{Logger: logger, cacheDir: cacheDir}, nil
}

// Save marshals output and saves it to a file whose name is generated according to roleArn and the current timestamp.
// On success, it returns the binary contents saved to the file as a byte slice. On failure, if the returned
// error wraps ErrInvalidCredential, the AWS credential is invalid. Otherwise only the write operation failed
// but the AWS credential is valid.
func (c *cacheSaveRetriever) Save(roleArn string, output *CredentialProcessOutput) (contents []byte, err error) {
	ts, err := time.Parse(expirationLayout, output.Expiration)
	if err != nil {
		return nil, fmt.Errorf("%w: Expiration %q is not of the right format: %w", ErrInvalidCredential, output.Expiration, err)
	}

	fileName := EncodeToFileName(roleArn, ts)
	filePath := filepath.Join(c.cacheDir, fileName)

	contents, err = json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to serialize CredentialProcessOutput: %w", ErrInvalidCredential, err)
	}

	if err = os.WriteFile(filePath, contents, 0600); err != nil {
		return contents, fmt.Errorf("failed to save credentials to cache file: %w", err)
	}

	return contents, nil
}

func (c *cacheSaveRetriever) Retrieve(roleArn string) (contents []byte, err error) {
	max := time.Now().Add(time.Minute * 10)
	actives := make(cacheFileSlice, 0)
	pattern := filepath.Join(c.cacheDir, fmt.Sprintf(`%s-*.json`, GetPrefix(roleArn)))

	cacheFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid file globbing pattern: %w", err)
	}

	var expiration time.Time

	for _, fullPath := range cacheFiles {
		if expiration, err = DecodeFromFileName(roleArn, filepath.Base(fullPath)); err != nil {
			c.deleteCacheFile(fullPath, "invalid")

			continue
		} else if expiration.Before(max) {
			c.deleteCacheFile(fullPath, "almost expired")

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
		c.deleteCacheFile(item.filePath, "older")
	}

	contents, err = os.ReadFile(actives[0].filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read active cache file %q: %w", actives[0].filePath, err)
	}

	return contents, nil
}

func (c *cacheSaveRetriever) deleteCacheFile(fullPath string, desc string) {
	if os.Remove(fullPath) != nil {
		c.Logger.Printf("Failed to delete %s cache file %q.\n", desc, fullPath)
	}
}
