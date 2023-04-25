package service

import (
	"context"
	transportHttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewImage)

type ImageInterface interface {
	ImageHandler(ctx context.Context, httpContext transportHttp.Context) (interface{}, error)
}
