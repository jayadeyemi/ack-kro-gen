package render

import (
	"context"
	"testing"

	"github.com/jayadeyemi/ack-kro-gen/internal/config"
)

func TestRenderDummyChart(t *testing.T) {
	ctx := context.Background()
	res, err := RenderChart(ctx, "internal/render/testdata/dummychart", config.GraphSpec{
		Service:     "dummy",
		Version:     "0.1.0",
		ReleaseName: "__KRO_NAME__",
		Namespace:   "__KRO_NS__",
		Image:       config.ImageSpec{Repository: "__KRO_IMAGE_REPO__", Tag: "__KRO_IMAGE_TAG__"},
		ServiceAccount: config.SASpec{
			Name:        "__KRO_SA_NAME__",
			Annotations: map[string]string{"eks.amazonaws.com/role-arn": "__KRO_IRSA_ARN__"},
		},
		Controller: config.ControllerSpec{
			LogLevel:  "__KRO_LOG_LEVEL__",
			LogDev:    "__KRO_LOG_DEV__",
			AWSRegion: "__KRO_AWS_REGION__",
		},
	})
	if err != nil { t.Fatal(err) }
	if len(res.RenderedFiles) == 0 { t.Fatal("no rendered files") }
}