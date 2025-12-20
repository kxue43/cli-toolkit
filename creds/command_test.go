package creds

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/awsdocs/aws-doc-sdk-examples/gov2/testtools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kxue43/cli-toolkit/cipher"
	"github.com/kxue43/cli-toolkit/terminal"
)

type (
	MockTerminal struct {
		r bytes.Buffer
		w bytes.Buffer
	}

	HomeDirMocker struct {
		TempDir string
		home    string
	}
)

func (fd *MockTerminal) Read(p []byte) (n int, err error) {
	return fd.r.Read(p)
}

func (fd *MockTerminal) Write(p []byte) (n int, err error) {
	return fd.w.Write(p)
}

func (m *HomeDirMocker) SetUp(t *testing.T) {
	t.Helper()

	var err error

	m.TempDir, err = os.MkdirTemp("", "cli-toolkit")
	require.NoError(t, err, "should be able to set up a temp directory for holding cache files during tests")

	m.home, err = os.UserHomeDir()
	require.NoError(t, err, "should be able to get user home directory")

	err = os.Setenv("HOME", m.TempDir)
	require.NoError(t, err, "should be able to set the HOME environment variable during tests")
}

func (m *HomeDirMocker) TearDown(t *testing.T) {
	t.Helper()

	err := os.Setenv("HOME", m.home)
	require.NoError(t, err, "should be able to reset HOME to its original value after tests")

	err = os.RemoveAll(m.TempDir)
	require.NoError(t, err, "should be able to remove temp directory after tests")
}

func TestAssumeRoleCmdRun(t *testing.T) {
	var aesKey [cipher.AesKeySize]byte

	_, err := generateKey(&aesKey)
	require.NoError(t, err, "should be able to generate a random encryption key")

	fromKeyring = func(key *[cipher.AesKeySize]byte) error {
		t.Helper()

		copy(key[:], aesKey[:])

		return nil
	}

	defer func() { fromKeyring = keyringGet }()

	hdm := HomeDirMocker{}

	hdm.SetUp(t)
	defer hdm.TearDown(t)

	roleArn := "role-arn"

	expiration := time.Now().Add(10 * time.Hour)

	var duration int32 = 3600

	t.Run("Happy path no cache", func(t *testing.T) {
		// Create an AssumeRoleCmd with some fields already filled with valid values.
		// We don't test CLI parsing in unit tests.
		cmd := AssumeRoleCmd{
			MFASerial:       "mfa-serial",
			Profile:         "profile",
			Region:          "us-east-1",
			RoleSessionName: "ToolkitCLI",
			DurationSeconds: int64(duration),
		}

		token := "123456"

		mockedTerminal := &MockTerminal{}

		_, err := mockedTerminal.r.WriteString(token + "\n")
		require.NoError(t, err, "should be able to write token to mocked TTY file descriptor")

		tty := terminal.NewTTY(mockedTerminal, "toolkit-assume-role: ", 0)

		dest := MockTerminal{}

		soutput := CredentialProcessOutput{
			AccessKeyId:     "access-key-id",
			SecretAccessKey: "secret-access-key",
			SessionToken:    "session-token",
			Expiration:      expiration.Format(time.RFC3339),
			Version:         1,
		}

		stubber := testtools.NewStubber()
		defer testtools.ExitTest(stubber, t)

		stubber.Add(testtools.Stub{
			OperationName: "AssumeRole",
			Input: &sts.AssumeRoleInput{
				DurationSeconds: &duration,
				RoleArn:         &roleArn,
				RoleSessionName: &cmd.RoleSessionName,
				SerialNumber:    &cmd.MFASerial,
				TokenCode:       &token,
			},
			Output: &sts.AssumeRoleOutput{
				Credentials: &types.Credentials{
					AccessKeyId:     &soutput.AccessKeyId,
					SecretAccessKey: &soutput.SecretAccessKey,
					SessionToken:    &soutput.SessionToken,
					Expiration:      &expiration,
				},
			},
			Error: nil,
		})

		ctx := context.Background()

		err = cmd.ValidateInputs([]string{roleArn})
		require.NoError(t, err, "fields of AssumeRoleCmd should validate without error")

		// Use a mocked credentials provider so that we can load config without error during tests.
		mockCredsProvider := credentials.NewStaticCredentialsProvider("dummy", "dummy", "dummy")

		cfg, err := config.LoadDefaultConfig(ctx, config.WithCredentialsProvider(mockCredsProvider), config.WithRegion(cmd.Region))
		require.NoError(t, err, "should be able to load config using a mocked credentials provider")

		err = cmd.Init(tty, cfg)
		require.NoError(t, err, "should be able to init caching without error")

		// Stub the STS client object.
		cmd.client = sts.NewFromConfig(*stubber.SdkConfig)

		err = cmd.Run(ctx, &dest)
		require.NoError(t, err, "should be able to run command without error")

		cacheFilePath := filepath.Join(hdm.TempDir, ".aws", "toolkit-cache", encodeToFileName(roleArn, expiration))

		info, err := os.Stat(cacheFilePath)
		require.NoError(t, err, "should be able to locate the cache file created by the Run method")

		assert.False(t, info.IsDir(), "the cache file created by the Run method should be a regular file, not a directory")

		rawContents, err := os.ReadFile(filepath.Clean(cacheFilePath))
		require.NoError(t, err, "should be able to read cache file raw content without error")

		rawContents, err = cmd.cacher.cipher.Decrypt(rawContents)
		require.NoError(t, err, "should be able to decrypt cache file without error")

		var sCachedContents CredentialProcessOutput

		err = json.Unmarshal(rawContents, &sCachedContents)
		require.NoError(t, err, "should be able to unmarshal decrypted cache file without error")

		assert.Equal(t, sCachedContents.AccessKeyId, soutput.AccessKeyId, "AccessKeyId from cache file should match STS call result")

		assert.Equal(t, sCachedContents.SecretAccessKey, soutput.SecretAccessKey, "SecretAccessKey from cache file should match STS call result")

		assert.Equal(t, sCachedContents.SessionToken, soutput.SessionToken, "SessionToken from cache file should match STS call result")

		assert.Equal(t, sCachedContents.Expiration, soutput.Expiration, "Expiration from cache file should match STS call result")

		assert.Equal(t, sCachedContents.Version, soutput.Version, "Version from cache file should be the right value of 1")

		assert.Equal(t, rawContents, dest.w.Bytes(), "outputs to stdout should be identical to decrypted cache file contents")
	})

	t.Run("Happy path cache hits", func(t *testing.T) {
		cmd := AssumeRoleCmd{
			MFASerial:       "mfa-serial",
			Profile:         "profile",
			Region:          "us-east-1",
			RoleSessionName: "ToolkitCLI",
			DurationSeconds: int64(duration),
		}

		mockedTerminal := &MockTerminal{}

		tty := terminal.NewTTY(mockedTerminal, "toolkit-assume-role: ", 0)

		dest := MockTerminal{}

		ctx := context.Background()

		err := cmd.ValidateInputs([]string{roleArn})
		require.NoError(t, err, "fields of AssumeRoleCmd should validate without error")

		// Use a mocked credentials provider so that we can load config without error during tests.
		mockCredsProvider := credentials.NewStaticCredentialsProvider("dummy", "dummy", "dummy")

		cfg, err := config.LoadDefaultConfig(ctx, config.WithCredentialsProvider(mockCredsProvider), config.WithRegion(cmd.Region))
		require.NoError(t, err, "should be able to load config using a mocked credentials provider")

		err = cmd.Init(tty, cfg)
		require.NoError(t, err, "should be able to init caching without error")

		// Stub the STS client object. When cache hits, the STS client shouldn't have been used at all.
		cmd.client = nil

		err = cmd.Run(ctx, &dest)
		require.NoError(t, err, "should be able to run command without error")

		cacheFilePath := filepath.Join(hdm.TempDir, ".aws", "toolkit-cache", encodeToFileName(roleArn, expiration))

		info, err := os.Stat(cacheFilePath)
		require.NoError(t, err, "should be able to locate the cache file created by the Run method")

		assert.False(t, info.IsDir(), "the cache file created by the Run method should be a regular file, not a directory")

		rawContents, err := os.ReadFile(filepath.Clean(cacheFilePath))
		require.NoError(t, err, "should be able to read cache file raw content without error")

		rawContents, err = cmd.cacher.cipher.Decrypt(rawContents)
		require.NoError(t, err, "should be able to decrypt cache file without error")

		var sCachedContents CredentialProcessOutput

		err = json.Unmarshal(rawContents, &sCachedContents)
		require.NoError(t, err, "should be able to unmarshal decrypted cache file without error")

		var stdoutContents CredentialProcessOutput

		err = json.Unmarshal(dest.w.Bytes(), &stdoutContents)
		require.NoError(t, err, "should be able to unmarshal outputs to stdout without error")

		assert.Equal(t, sCachedContents.AccessKeyId, stdoutContents.AccessKeyId, "AccessKeyId from cache file should match that from stdout")

		assert.Equal(t, sCachedContents.SecretAccessKey, stdoutContents.SecretAccessKey, "SecretAccessKey from cache file should match that from stdout")

		assert.Equal(t, sCachedContents.SessionToken, stdoutContents.SessionToken, "SessionToken from cache file should match that from stdout")

		assert.Equal(t, sCachedContents.Expiration, stdoutContents.Expiration, "Expiration from cache file should match that from stdout")

		assert.Equal(t, sCachedContents.Version, stdoutContents.Version, "Version from cache file should match that from stdout")
	})
}
