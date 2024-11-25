package fake

import (
	"context"

	"github.com/docker/distribution"
	"github.com/openshift/hypershift/support/thirdparty/library-go/pkg/image/dockerv1client"
)

type FakeImageMetadataProvider struct {
	Result   *dockerv1client.DockerImageConfig
	Manifest distribution.Manifest
}

func (f *FakeImageMetadataProvider) ImageMetadata(ctx context.Context, imageRef string, pullSecret []byte) (*dockerv1client.DockerImageConfig, error) {
	return f.Result, nil
}

func (f *FakeImageMetadataProvider) GetManifest(ctx context.Context, imageRef string, pullSecret []byte) (distribution.Manifest, error) {
	return f.Manifest, nil
}
