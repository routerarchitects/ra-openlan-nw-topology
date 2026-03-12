package service_rpc

import (
	"log/slog"

	"github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc/analytics"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc/common"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/service_rpc/owsec"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
)

type ServiceRpc struct {
	deps *common.ServiceRPCBase
}

func NewServiceRpc(
	discovery *servicediscovery.Discovery,
	cfg ServiceRpcConfig,
	logger *slog.Logger,
) *ServiceRpc {
	return &ServiceRpc{
		deps: common.NewServiceRPCBase(discovery, cfg.TLSRootCA, cfg.Timeout, cfg.InternalName, logger),
	}
}

func (f *ServiceRpc) AnalyticsClient() *analytics.AnalyticsClient {
	return analytics.NewAnalyticsClient(f.deps)
}

func (f *ServiceRpc) Validator() *owsec.Validator {
	return owsec.NewValidator(f.deps)
}
