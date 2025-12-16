package auth

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
)

type (
	MockFileDescriptor struct {
		r bytes.Buffer
		w bytes.Buffer
	}

	HomeDirMocker struct {
		TempDir string
		home    string
	}
)

func (fd *MockFileDescriptor) Read(p []byte) (n int, err error) {
	return fd.r.Read(p)
}

func (fd *MockFileDescriptor) Write(p []byte) (n int, err error) {
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
	fromKeyring = func() ([]byte, error) {
		t.Helper()

		key, _, err := generateKey(keySize)
		require.NoError(t, err, "should be able to generate a random encryption key")

		return key, nil
	}

	defer func() { fromKeyring = keyringGet }()

	tty := MockFileDescriptor{}
	dest := MockFileDescriptor{}

	hdm := HomeDirMocker{}

	hdm.SetUp(t)
	defer hdm.TearDown(t)

	// Create an AssumeRoleCmd with some fields already filled with valid values.
	// We don't test CLI parsing in unit tests.
	cmd := AssumeRoleCmd{
		MFASerial:       "mfa-serial",
		Profile:         "profile",
		Region:          "us-east-1",
		RoleSessionName: "ToolkitCLI",
		DurationSeconds: 3600,
	}

	roleArn := "role-arn"

	token := "123456"

	expiration := time.Now()

	duration := int32(cmd.DurationSeconds)

	soutput := CredentialProcessOutput{
		AccessKeyId:     "access-key-id",
		SecretAccessKey: "secret-access-key",
		SessionToken:    "session-token",
		Expiration:      expiration.Format(time.RFC3339),
		Version:         1,
	}

	stubber := testtools.NewStubber()

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

	_, err := tty.r.WriteString(token + "\n")
	require.NoError(t, err, "should be able to write token to mocked TTY file descriptor")

	ctx := context.Background()

	err = cmd.ValidateInputs([]string{roleArn})
	require.NoError(t, err, "fields of AssumeRoleCmd should validate without error")

	// Use a mocked credentials provider so that we can load config without error during tests.
	mockCredsProvider := credentials.NewStaticCredentialsProvider("dummy", "dummy", "dummy")

	cfg, err := config.LoadDefaultConfig(ctx, config.WithCredentialsProvider(mockCredsProvider), config.WithRegion(cmd.Region))
	require.NoError(t, err, "should be able to load config using a mocked credentials provider")

	err = cmd.InitCache(&tty, cfg)
	require.NoError(t, err, "should be able to init caching without error")

	// Stub the STS client object.
	cmd.client = sts.NewFromConfig(*stubber.SdkConfig)

	err = cmd.Run(ctx, &dest)
	require.NoError(t, err, "should be able to run command without error")

	cacheFilePath := filepath.Join(hdm.TempDir, ".aws", "toolkit-cache", EncodeToFileName(roleArn, expiration))

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
}
