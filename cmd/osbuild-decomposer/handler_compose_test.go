package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	main "github.com/osbuild/osbuild-composer/cmd/osbuild-decomposer"
	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
)

func newClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
}

func TestUnkownEndpoint(t *testing.T) {
	socketPath, _ := runTestServer(t)

	endpoint := "http://localhost/api/v1/unknown"
	rsp, err := newClient(socketPath).Get(endpoint)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, 404, rsp.StatusCode)
}

func TestBuildMustPOST(t *testing.T) {
	socketPath, _ := runTestServer(t)

	endpoint := "http://localhost/api/image-builder-composer/v2/compose"
	rsp, err := newClient(socketPath).Get(endpoint)
	assert.NoError(t, err)
	defer rsp.Body.Close()
	assert.Equal(t, 405, rsp.StatusCode)
	//assert.Equal(t, "handlerBuild called on /api/v1/build", loggerHook.LastEntry().Message)
}

func TestBuildIntegration(t *testing.T) {
	socketPath, baseResultDir := runTestServer(t)
	endpoint := "http://localhost/api/image-builder-composer/v2/compose"

	restore := main.MockIcliBinary(t, `#!/bin/sh -e
# we run in the output dir
echo "$0" "$@" > ibcli.args

# simulate build time
sleep 0.5

touch "fake-disk" > disk.img
`)
	defer restore()

	fakeComposeRequest := `
{
  "distribution": "centos-9",
  "image_request": {
    "image_type":"qcow2"
  }
}`

	// run three builds
	var ids []string
	for i := 0; i < 3; i++ {
		buf := bytes.NewBufferString(fakeComposeRequest)
		rsp, err := newClient(socketPath).Post(endpoint, "application/json", buf)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusCreated, rsp.StatusCode)
		var composeId v2.ComposeId
		err = json.NewDecoder(rsp.Body).Decode(&composeId)
		assert.NoError(t, err)
		assert.Equal(t, composeId.Kind, "ComposeID")
		ids = append(ids, composeId.Id.String())
	}

	for _, id := range ids {
		resultDir := filepath.Join(baseResultDir, id)
		for {
			if _, err := os.Stat(filepath.Join(resultDir, "disk.img")); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		callArgs, err := os.ReadFile(filepath.Join(resultDir, "ibcli.args"))
		assert.NoError(t, err)
		assert.Regexp(t, `image-builder build -v --with-buildlog --blueprint .* --output-dir .* --distro centos-9 qcow2`, string(callArgs))

		assert.True(t, main.NewBuildResult(resultDir).Good())
		assert.False(t, main.NewBuildResult(resultDir).Bad())
		assert.False(t, main.NewBuildResult(resultDir).Unknown())
	}

	// check status
	statusEndpoint := fmt.Sprintf("http://localhost/api/image-builder-composer/v2/composes/%s", ids[0])
	rsp, err := newClient(socketPath).Get(statusEndpoint)
	assert.NoError(t, err)
	var composeStatus v2.ComposeStatus
	err = json.NewDecoder(rsp.Body).Decode(&composeStatus)
	assert.NoError(t, err)
	assert.Equal(t, v2.ComposeStatusValueSuccess, composeStatus.Status)

	// check status
	composesEndpoint := "http://localhost/api/image-builder-composer/v2/composes"
	rsp, err = newClient(socketPath).Get(composesEndpoint)
	assert.NoError(t, err)
	var composes []v2.ComposeStatus
	err = json.NewDecoder(rsp.Body).Decode(&composes)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(composes))
}
