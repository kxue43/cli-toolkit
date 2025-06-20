package auth

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	require.NoError(t, err)

	m.home, err = os.UserHomeDir()
	require.NoError(t, err)

	err = os.Setenv("HOME", m.TempDir)
	require.NoError(t, err)
}

func (m *HomeDirMocker) TearDown(t *testing.T) {
	t.Helper()

	err := os.Setenv("HOME", m.home)
	require.NoError(t, err)

	err = os.RemoveAll(m.TempDir)
	require.NoError(t, err)
}

func TestAssumeRoleCmdRun(t *testing.T) {
	fd := MockFileDescriptor{}
	dest := MockFileDescriptor{}

	hdm := HomeDirMocker{}
	hdm.SetUp(t)
	defer hdm.TearDown(t)

	stubber := testtools.NewStubber()
	stubbedClient := sts.NewFromConfig(*stubber.SdkConfig)

	ci := ClientInteractor{&fd}
	logger := ci.NewLogger("toolkit: ", 0)

	cache, err := NewCacheSaveRetriever(logger)
	require.NoError(t, err)

	cmd := AssumeRoleCmd{
		ci:              ci,
		cache:           cache,
		stsClient:       stubbedClient,
		RoleArn:         "role-arn",
		MFASerial:       "mfa-serial",
		Profile:         "profile",
		Region:          "us-east-1",
		RoleSessionName: "ToolkitCLI",
		DurationSeconds: 3600,
	}

	token := "123456"
	expiration := time.Now()
	duration := int32(cmd.DurationSeconds)

	soutput := CredentialProcessOutput{
		AccessKeyId:     "access-key-id",
		SecretAccessKey: "secret-access-key",
		SessionToken:    "session-token",
		Expiration:      expiration.Format(expirationLayout),
		Version:         1,
	}

	stubber.Add(testtools.Stub{
		OperationName: "AssumeRole",
		Input: &sts.AssumeRoleInput{
			DurationSeconds: &duration,
			RoleArn:         &cmd.RoleArn,
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

	_, err = fd.r.WriteString(token + "\n")
	require.NoError(t, err)

	err = cmd.Run(&dest)
	require.NoError(t, err)

	cacheFilePath := filepath.Join(hdm.TempDir, ".aws", "toolkit-cache", EncodeToFileName(cmd.RoleArn, expiration))
	info, err := os.Stat(cacheFilePath)

	require.NoError(t, err)
	require.True(t, !info.IsDir())

	var output CredentialProcessOutput

	contents, err := os.ReadFile(filepath.Clean(cacheFilePath))
	require.NoError(t, err)

	err = json.Unmarshal(contents, &output)
	require.NoError(t, err)

	assert.Equal(t, output.AccessKeyId, soutput.AccessKeyId)
	assert.Equal(t, output.SecretAccessKey, soutput.SecretAccessKey)
	assert.Equal(t, output.SessionToken, soutput.SessionToken)
	assert.Equal(t, output.Expiration, soutput.Expiration)
	assert.Equal(t, output.Version, soutput.Version)
	assert.Equal(t, contents, dest.w.Bytes())
}
