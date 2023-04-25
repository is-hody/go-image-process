//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/google/wire"
	"go-image-process/internal/conf"
	"go-image-process/internal/server"
	"go-image-process/internal/service"
)

// initApp init kratos application.
func initApp(*conf.Bootstrap) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, service.ProviderSet, newApp))
}
