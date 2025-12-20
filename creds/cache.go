package creds

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/kxue43/cli-toolkit/cipher"
)

type (
	CredentialProcessOutput struct {
		AccessKeyId     string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
		SessionToken    string `json:"SessionToken"`
		Expiration      string `json:"Expiration"`
		Version         int    `json:"Version"`
	}

	cacher struct {
		logger   logger
		cipher   *cipher.AesGcm
		cacheDir string
	}

	cacheFile struct {
		expiration time.Time
		filePath   string
	}

	cacheFileSlice []*cacheFile
)

var (
	ErrCacheInit         = errors.New("cache initialization failure")
	ErrCacheSave         = errors.New("failed to save cache file")
	ErrInvalidCredential = errors.New("invalid AWS credential")
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

func getPrefix(s string) string {
	h := sha1.Sum([]byte(s))

	return hex.EncodeToString(h[:])[0:7]
}

func encodeToFileName(roleArn string, ts time.Time) string {
	return fmt.Sprintf("%s-%s", getPrefix(roleArn), strconv.FormatInt(ts.Unix(), 10))
}

func decodeFromFileName(roleArn, fileName string) (ts time.Time, err error) {
	regex := regexp.MustCompile(fmt.Sprintf(`^%s-(\d+)$`, getPrefix(roleArn)))

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

// Non-nil returned error wraps [ErrCacheInit].
func newCacher(logger logger, fn cipher.KeyFunc) (*cacher, error) {
	aes, err := cipher.NewAesGcm(fn)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrCacheInit, err.Error())
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("%w: could not locate user home directory", ErrCacheInit)
	}

	cacheDir := filepath.Join(home, ".aws", "toolkit-cache")

	info, err := os.Stat(cacheDir)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(cacheDir, 0750); err != nil {
			return nil, fmt.Errorf("%w: failed to create cache directory", ErrCacheInit)
		}

		return &cacher{logger: logger, cacheDir: cacheDir, cipher: aes}, nil
	} else if err != nil {
		return nil, fmt.Errorf("%w: failed to locate cache directory: %s", ErrCacheInit, err.Error())
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: cache directory is already a file", ErrCacheInit)
	}

	return &cacher{logger: logger, cacheDir: cacheDir, cipher: aes}, nil
}

// Non-nil returned error wraps [ErrInvalidCredential] or [ErrCacheSave].
// contents is valid for use as long as it's not nil.
func (c *cacher) Save(roleArn string, output *CredentialProcessOutput) (contents []byte, err error) {
	ts, err := time.Parse(time.RFC3339, output.Expiration)
	if err != nil {
		return nil, fmt.Errorf("%w: expiration %q is not of the right format: %s", ErrInvalidCredential, output.Expiration, err.Error())
	}

	contents, err = json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to serialize CredentialProcessOutput: %s", ErrInvalidCredential, err.Error())
	}

	filePath := filepath.Join(c.cacheDir, encodeToFileName(roleArn, ts))

	encrypted, err := c.cipher.Encrypt(contents)
	if err != nil {
		return contents, fmt.Errorf("%w: failed to encrypt before saving: %s", ErrCacheSave, err.Error())
	}

	if err = os.WriteFile(filePath, encrypted, 0600); err != nil {
		return contents, fmt.Errorf("%w: failed to write to disk: %s", ErrCacheSave, err.Error())
	}

	return contents, nil
}

// Retrieve tries to retrieve AWS credentials from cache files.
// It succeeded if and only if the returned byte slice is not nil.
func (c *cacher) Retrieve(roleArn string) (contents []byte) {
	max := time.Now().Add(time.Minute * 10)
	actives := make(cacheFileSlice, 0)
	pattern := filepath.Join(c.cacheDir, fmt.Sprintf(`%s-*`, getPrefix(roleArn)))

	cacheFiles, err := filepath.Glob(pattern)
	if err != nil {
		c.logger.Printf("invalid file globbing pattern: %s\n", err)

		return nil
	}

	var expiration time.Time

	for _, fullPath := range cacheFiles {
		if expiration, err = decodeFromFileName(roleArn, filepath.Base(fullPath)); err != nil {
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
		return nil
	}

	sort.Sort(actives)

	for _, item := range actives[1:] {
		c.deleteCacheFile(item.filePath, "older")
	}

	contents, err = os.ReadFile(actives[0].filePath)
	if err != nil {
		c.logger.Printf("failed to read active cache file %q: %s\n", actives[0].filePath, err)

		return nil
	}

	contents, err = c.cipher.Decrypt(contents)
	if err != nil {
		c.logger.Printf("failed to decrypt cache file %q: %s\n", actives[0].filePath, err)

		return nil
	}

	return contents
}

func (c *cacher) deleteCacheFile(fullPath string, desc string) {
	if os.Remove(fullPath) != nil {
		c.logger.Printf("Failed to delete %s cache file %q.\n", desc, fullPath)
	}
}
