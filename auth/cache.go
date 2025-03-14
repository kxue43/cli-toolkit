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

var fileNameDecodingErr = errors.New("file name is not of the right format; expecting `${prefix}-${UnixSeconds}.json`")
var expirationLayout = time.RFC3339

type (
	CredentialProcessOutput struct {
		Version         int
		AccessKeyId     string
		SecretAccessKey string
		SessionToken    string
		Expiration      string
	}

	fileNameEncoderDecoder struct {
		roleArn string
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

func (f *fileNameEncoderDecoder) GetPrefix() string {
	h := sha1.Sum([]byte(f.roleArn))
	return hex.EncodeToString(h[:])[0:7]
}

func (f *fileNameEncoderDecoder) Encode(ts time.Time) string {
	return fmt.Sprintf("%s-%s.json", f.GetPrefix(), strconv.FormatInt(ts.Unix(), 10))
}

func (f *fileNameEncoderDecoder) Decode(fileName string) (time.Time, error) {
	var ts time.Time
	regex := regexp.MustCompile(fmt.Sprintf(`^%s-(\d+)\.json$`, f.GetPrefix()))
	matches := regex.FindStringSubmatch(fileName)
	if matches == nil {
		return ts, fmt.Errorf("%q: %w", fileName, fileNameDecodingErr)
	}
	unixSec, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return ts, fmt.Errorf("could not parse Unix second %q as integer: %w", matches[1], err)
	}
	ts = time.Unix(unixSec, 0)
	return ts, nil
}

func NewCacheRetrieverSaver(logger *log.Logger) *cacheRetrieverSaver {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Fatalf("Could not locate user home directory: %s.\n", err)
	}
	cacheDir := filepath.Join(home, ".aws", "toolkit-cache")
	return &cacheRetrieverSaver{Logger: logger, cacheDir: cacheDir}
}

func (c *cacheRetrieverSaver) ensureCacheDir() {
	info, err := os.Stat(c.cacheDir)
	if err == nil && !info.IsDir() {
		err := os.Remove(c.cacheDir)
		if err != nil {
			c.Logger.Fatalf("%q is a file and cannot be deleted: %s.\n", c.cacheDir, err)
		}
	}
	if os.IsNotExist(err) {
		err := os.MkdirAll(c.cacheDir, 0755)
		if err != nil {
			c.Logger.Fatalf("Cache directory %q does not exist and cannot be created: %s.\n", c.cacheDir, err)
		}
	}
}

func (c *cacheRetrieverSaver) Save(roleArn string, output *CredentialProcessOutput) []byte {
	c.ensureCacheDir()
	ts, err := time.Parse(expirationLayout, output.Expiration)
	if err != nil {
		c.Logger.Printf("CredentialProcessOutput.Expiration %q is not of the right format: %s.\n", output.Expiration, err)
		return nil
	}
	encoder := fileNameEncoderDecoder{roleArn: roleArn}
	fileName := encoder.Encode(ts)
	filePath := filepath.Join(c.cacheDir, fileName)
	contents, err := json.Marshal(output)
	if err != nil {
		c.Logger.Printf("failed to serialize CredentialProcessOutput: %s.\n", err)
		return nil
	}
	err = os.WriteFile(filePath, contents, 0644)
	if err != nil {
		c.Logger.Printf("failed to save credentials to file cache: %s.\n", err)
		info, err := os.Stat(filePath)
		if err == nil && !info.IsDir() {
			err = os.Remove(filePath)
			if err != nil {
				c.Logger.Printf("failed to remove partial cache file %q: %s.\n", filePath, err)
			} else {
				c.Logger.Printf("removed partially written cache file %q.\n", filePath)
			}
		}
	}
	return contents
}

func (c *cacheRetrieverSaver) Retrieve(roleArn string) ([]byte, bool) {
	info, err := os.Stat(c.cacheDir)
	if !(err == nil && info.IsDir()) {
		return nil, false
	}
	decoder := fileNameEncoderDecoder{roleArn: roleArn}
	prefix := decoder.GetPrefix()

	max := time.Now().Add(time.Minute * 10)
	actives := make(cacheFileSlice, 0)
	pattern := filepath.Join(c.cacheDir, fmt.Sprintf(`%s-*.json`, prefix))
	cacheFiles, err := filepath.Glob(pattern)
	if err != nil {
		c.Logger.Fatalf("Invalid file globbing pattern %q.\n", pattern)
	}
	for _, fullPath := range cacheFiles {
		fileName := filepath.Base(fullPath)
		expiration, err := decoder.Decode(fileName)
		if err != nil {
			err = os.Remove(fullPath)
			if err != nil {
				c.Logger.Fatalf("Failed to delete invalid cache file %q.\n", fullPath)
			}
			continue
		}
		if expiration.Before(max) {
			err = os.Remove(fullPath)
			if err != nil {
				c.Logger.Fatalf("Failed to delete almost expired cache file %q.\n", fullPath)
			}
			continue
		}
		actives = append(actives, &cacheFile{expiration: expiration, filePath: fullPath})
	}
	if len(actives) == 0 {
		return nil, false
	}
	sort.Sort(actives)
	for _, item := range actives[1:] {
		err = os.Remove(item.filePath)
		if err != nil {
			c.Logger.Fatalf("Failed to delete older cache file %q.\n", item.filePath)
		}
	}
	contents, err := os.ReadFile(actives[0].filePath)
	if err != nil {
		c.Logger.Fatalf("failed to read active cache file %q.\n", actives[0].filePath)
	}
	return contents, true
}
