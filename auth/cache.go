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

	Cacher struct {
		logger   *log.Logger
		cacheDir string
		cipher   Cipher
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

func GetPrefix(s string) string {
	h := sha1.Sum([]byte(s))

	return hex.EncodeToString(h[:])[0:7]
}

func EncodeToFileName(roleArn string, ts time.Time) string {
	return fmt.Sprintf("%s-%s", GetPrefix(roleArn), strconv.FormatInt(ts.Unix(), 10))
}

func DecodeFromFileName(roleArn, fileName string) (ts time.Time, err error) {
	regex := regexp.MustCompile(fmt.Sprintf(`^%s-(\d+)\$`, GetPrefix(roleArn)))

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
func NewCacher(logger *log.Logger, cipher Cipher) (*Cacher, error) {
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

		return &Cacher{logger: logger, cacheDir: cacheDir, cipher: cipher}, nil
	} else if err != nil {
		return nil, fmt.Errorf("%w: failed to locate cache directory: %s", ErrCacheInit, err.Error())
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: cache directory is already a file", ErrCacheInit)
	}

	return &Cacher{logger: logger, cacheDir: cacheDir, cipher: cipher}, nil
}

// Non-nil returned error wraps [ErrInvalidCredential] or [ErrCacheSave].
func (c *Cacher) Save(roleArn string, output *CredentialProcessOutput) (contents []byte, err error) {
	ts, err := time.Parse(time.RFC3339, output.Expiration)
	if err != nil {
		return nil, fmt.Errorf("%w: expiration %q is not of the right format: %s", ErrInvalidCredential, output.Expiration, err.Error())
	}

	fileName := EncodeToFileName(roleArn, ts)
	filePath := filepath.Join(c.cacheDir, fileName)

	contents, err = json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to serialize CredentialProcessOutput: %s", ErrInvalidCredential, err.Error())
	}

	encrypted, err := c.cipher.Encrypt(contents)
	if err != nil {
		return contents, fmt.Errorf("%w: failed to encrypt before saving: %s", ErrCacheSave, err.Error())
	}

	if err = os.WriteFile(filePath, encrypted, 0600); err != nil {
		return contents, fmt.Errorf("%w: failed to write to disk: %s", ErrCacheSave, err.Error())
	}

	return contents, nil
}

func (c *Cacher) Retrieve(roleArn string) (contents []byte) {
	max := time.Now().Add(time.Minute * 10)
	actives := make(cacheFileSlice, 0)
	pattern := filepath.Join(c.cacheDir, fmt.Sprintf(`%s-*`, GetPrefix(roleArn)))

	cacheFiles, err := filepath.Glob(pattern)
	if err != nil {
		c.logger.Printf("invalid file globbing pattern: %s\n", err)

		return nil
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

func (c *Cacher) deleteCacheFile(fullPath string, desc string) {
	if os.Remove(fullPath) != nil {
		c.logger.Printf("Failed to delete %s cache file %q.\n", desc, fullPath)
	}
}
