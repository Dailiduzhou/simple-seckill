//go:build wireinject
// +build wireinject

package main

import (
	"seckill/app/product/internal/biz"
	"seckill/app/product/internal/conf"
	"seckill/app/product/internal/data"
	"seckill/app/product/internal/server"
	"seckill/app/product/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

func wireApp(*conf.Server, *conf.Data, *conf.Registry, *conf.Dtm, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
