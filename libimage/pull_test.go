package libimage

import (
	"context"
	"fmt"
	"os"
	goruntime "runtime"
	"testing"

	"github.com/containers/common/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPull(t *testing.T) {
	runtime, cleanup := testNewRuntime(t)
	defer cleanup()
	ctx := context.Background()
	pullOptions := &PullOptions{}
	pullOptions.Writer = os.Stdout

	// Make sure that parsing errors of the daemon transport are returned
	// and that we do not fallthrough attempting to pull the specified
	// string as an image from a registry.
	_, err := runtime.Pull(ctx, "docker-daemon:alpine", config.PullPolicyAlways, pullOptions)
	require.Error(t, err, "return parsing error from daemon transport")

	for _, test := range []struct {
		input       string
		expectError bool
		numImages   int
		names       []string
	}{
		// DOCKER ARCHIVE
		{"docker-archive:testdata/docker-name-only.tar.xz", false, 1, []string{"localhost/pretty-empty:latest"}},
		{"docker-archive:testdata/docker-registry-name.tar.xz", false, 1, []string{"example.com/empty:latest"}},
		{"docker-archive:testdata/docker-two-names.tar.xz", false, 2, []string{"example.com/empty:latest", "localhost/pretty-empty:latest"}},
		{"docker-archive:testdata/docker-two-images.tar.xz", true, 0, nil}, // LOAD must be used here
		{"docker-archive:testdata/docker-unnamed.tar.xz", false, 1, []string{"ec9293436c2e66da44edb9efb8d41f6b13baf62283ebe846468bc992d76d7951"}},

		// OCI ARCHIVE
		{"oci-archive:testdata/oci-name-only.tar.gz", false, 1, []string{"localhost/pretty-empty:latest"}},
		{"oci-archive:testdata/oci-non-docker-name.tar.gz", true, 0, nil},
		{"oci-archive:testdata/oci-registry-name.tar.gz", false, 1, []string{"example.com/empty:latest"}},
		{"oci-archive:testdata/oci-unnamed.tar.gz", false, 1, []string{"5c8aca8137ac47e84c69ae93ce650ce967917cc001ba7aad5494073fac75b8b6"}},

		// REGISTRY
		{"alpine", false, 1, []string{"docker.io/library/alpine:latest"}},
		{"docker://alpine", false, 1, []string{"docker.io/library/alpine:latest"}},
		{"docker.io/library/alpine", false, 1, []string{"docker.io/library/alpine:latest"}},
		{"docker://docker.io/library/alpine", false, 1, []string{"docker.io/library/alpine:latest"}},
		{"quay.io/libpod/alpine@sha256:634a8f35b5f16dcf4aaa0822adc0b1964bb786fca12f6831de8ddc45e5986a00", false, 1, []string{"quay.io/libpod/alpine@sha256:634a8f35b5f16dcf4aaa0822adc0b1964bb786fca12f6831de8ddc45e5986a00"}},
		{"quay.io/libpod/alpine:pleaseignorethistag@sha256:634a8f35b5f16dcf4aaa0822adc0b1964bb786fca12f6831de8ddc45e5986a00", false, 1, []string{"quay.io/libpod/alpine@sha256:634a8f35b5f16dcf4aaa0822adc0b1964bb786fca12f6831de8ddc45e5986a00"}},

		// DIR
		{"dir:testdata/scratch-dir-5pec!@L", false, 1, []string{"61e17f84d763cc086d43c67dcf4cdbd69f9224c74e961c53b589b70499eac443"}},
	} {
		pulledImages, err := runtime.Pull(ctx, test.input, config.PullPolicyAlways, pullOptions)
		if test.expectError {
			require.Error(t, err, test.input)
			continue
		}
		require.NoError(t, err, test.input)
		require.Len(t, pulledImages, test.numImages)

		// Now lookup an image with the expected name and compare IDs.
		image, resolvedName, err := runtime.LookupImage(test.names[0], nil)
		require.NoError(t, err, test.input)
		require.Equal(t, test.names[0], resolvedName, fmt.Sprintf("%v", image.Names()))
		require.Equal(t, pulledImages[0].ID(), image.ID(), test.input)

		// Now remove the image.
		rmReports, rmErrors := runtime.RemoveImages(ctx, test.names, &RemoveImagesOptions{Force: true})
		require.Len(t, rmErrors, 0)
		require.Len(t, rmReports, 1)
		assert.Equal(t, image.ID(), rmReports[0].ID)
		assert.True(t, rmReports[0].Removed)
	}
}

func TestPullPlatforms(t *testing.T) {
	runtime, cleanup := testNewRuntime(t)
	defer cleanup()
	ctx := context.Background()
	pullOptions := &PullOptions{}
	pullOptions.Writer = os.Stdout

	localArch := goruntime.GOARCH
	localOS := goruntime.GOOS

	withTag := "busybox:musl"

	pulledImages, err := runtime.Pull(ctx, withTag, config.PullPolicyAlways, pullOptions)
	require.NoError(t, err, "pull busybox")
	require.Len(t, pulledImages, 1)

	image, _, err := runtime.LookupImage(withTag, nil)
	require.NoError(t, err, "lookup busybox")
	require.NotNil(t, image, "lookup busybox")

	_, _, err = runtime.LookupImage("busybox", nil)
	require.Error(t, err, "untagged image resolves to non-existent :latest")

	image, _, err = runtime.LookupImage(withTag, &LookupImageOptions{Architecture: localArch})
	require.NoError(t, err, "lookup busybox - by local arch")
	require.NotNil(t, image, "lookup busybox - by local arch")

	image, _, err = runtime.LookupImage(withTag, &LookupImageOptions{OS: localOS})
	require.NoError(t, err, "lookup busybox - by local arch")
	require.NotNil(t, image, "lookup busybox - by local arch")

	_, _, err = runtime.LookupImage(withTag, &LookupImageOptions{Architecture: "bogus"})
	require.Error(t, err, "lookup busybox - bogus arch")

	_, _, err = runtime.LookupImage(withTag, &LookupImageOptions{OS: "bogus"})
	require.Error(t, err, "lookup busybox - bogus OS")

	pullOptions.Architecture = "arm"
	pulledImages, err = runtime.Pull(ctx, withTag, config.PullPolicyAlways, pullOptions)
	require.NoError(t, err, "pull busybox - arm")
	require.Len(t, pulledImages, 1)
	pullOptions.Architecture = ""

	image, _, err = runtime.LookupImage(withTag, &LookupImageOptions{Architecture: "arm"})
	require.NoError(t, err, "lookup busybox - by arm")
	require.NotNil(t, image, "lookup busybox - by arm")

	pullOptions.Architecture = "aarch64"
	pulledImages, err = runtime.Pull(ctx, withTag, config.PullPolicyAlways, pullOptions)
	require.NoError(t, err, "pull busybox - aarch64")
	require.Len(t, pulledImages, 1)
}

func TestPullPlatformsWithEmptyRegistriesConf(t *testing.T) {
	runtime, cleanup := testNewRuntime(t, testNewRuntimeOptions{registriesConfPath: "/dev/null"})
	defer cleanup()
	ctx := context.Background()
	pullOptions := &PullOptions{}
	pullOptions.Writer = os.Stdout

	localArch := goruntime.GOARCH
	localOS := goruntime.GOOS

	imageName := "quay.io/libpod/busybox"
	newTag := "crazy:train"

	pulledImages, err := runtime.Pull(ctx, imageName, config.PullPolicyAlways, pullOptions)
	require.NoError(t, err, "pull "+imageName)
	require.Len(t, pulledImages, 1)

	err = pulledImages[0].Tag(newTag)
	require.NoError(t, err, "tag")

	// See containers/podman/issues/12707: a custom platform will enforce
	// pulling via newer. Older versions enforced always which can lead to
	// errors.
	pullOptions.OS = localOS
	pullOptions.Architecture = localArch
	pulledImages, err = runtime.Pull(ctx, newTag, config.PullPolicyMissing, pullOptions)
	require.NoError(t, err, "pull "+newTag)
	require.Len(t, pulledImages, 1)
}

func TestPullPolicy(t *testing.T) {
	runtime, cleanup := testNewRuntime(t)
	defer cleanup()
	ctx := context.Background()
	pullOptions := &PullOptions{}

	pulledImages, err := runtime.Pull(ctx, "alpine", config.PullPolicyNever, pullOptions)
	require.Error(t, err, "Never pull different arch alpine")
	require.Nil(t, pulledImages, "lookup alpine")

	pulledImages, err = runtime.Pull(ctx, "alpine", config.PullPolicyNewer, pullOptions)
	require.NoError(t, err, "Newer pull different arch alpine")
	require.NotNil(t, pulledImages, "lookup alpine")

	pulledImages, err = runtime.Pull(ctx, "alpine", config.PullPolicyNever, pullOptions)
	require.NoError(t, err, "Never pull different arch alpine")
	require.NotNil(t, pulledImages, "lookup alpine")

}
