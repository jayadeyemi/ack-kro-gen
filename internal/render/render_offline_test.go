package render

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

func TestRenderDummyChart(t *testing.T) {
	ctx := context.Background()
	chartPath := filepath.Join("testdata", "dummychart")
	res, err := RenderChart(ctx, chartPath, config.GraphSpec{
		Service:     "dummy",
		Version:     "0.1.0",
		ReleaseName: "__KRO_NAME__",
		Namespace:   "__KRO_NAMESPACE__",
		AWS: config.AWSSpec{
			Region: "__KRO_AWS_REGION__",
		},
		Image: config.ImageSpec{Repository: "__KRO_IMAGE_REPOSITORY__", Tag: "__KRO_IMAGE_TAG__"},
		ServiceAccount: config.SASpec{
			Name:        "__KRO_SA_NAME__",
			Annotations: map[string]string{"eks.amazonaws.com/role-arn": "__KRO_IRSA_ARN__"},
		},
		Controller: config.ControllerSpec{
			LogLevel:       "__KRO_LOG_LEVEL__",
			LogDev:         "__KRO_LOG_DEV__",
			WatchNamespace: "__KRO_WATCH_NAMESPACE__",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.RenderedFiles) == 0 {
		t.Fatal("no rendered files")
	}
}
