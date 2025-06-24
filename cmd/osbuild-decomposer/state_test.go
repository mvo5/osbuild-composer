package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
)

func TestInvalidComposes(t *testing.T) {
	testCases := []struct {
		name    string
		req     *v2.ComposeRequest
		wantErr bool
	}{
		{
			name: "invalid - image request & image requests objects",
			req: &v2.ComposeRequest{
				Distribution: "centos-9",
				ImageRequest: &v2.ImageRequest{
					ImageType: "qcow2",
				},
				ImageRequests: &[]v2.ImageRequest{
					{
						ImageType: "qcow2",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid - multiple image requests objects",
			req: &v2.ComposeRequest{
				Distribution: "centos-9",
				ImageRequests: &[]v2.ImageRequest{
					{
						ImageType: "qcow2",
					},
					{
						ImageType: "ami",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr {
				err := runIbcli(context.Background(), "", tt.req)
				assert.Error(t, err)
			}
		})
	}
}
