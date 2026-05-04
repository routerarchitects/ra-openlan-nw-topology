package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/ow-common-mods/fiber/middleware/auth"
	"github.com/routerarchitects/ow-common-mods/servicerpc/owsec"
)

type TopologyAuth struct {
	publicAuth  fiber.Handler
	privateAuth fiber.Handler
}

func NewTopologyAuth(
	publicCfg auth.PublicAuthConfig,
	privateCfg auth.InternalAPIKeyConfig,
	validator *owsec.SecurityClient,
) (*TopologyAuth, error) {
	if publicCfg.Validator == nil {
		publicCfg.Validator = validator
	}

	publicAuth, err := auth.RequirePublicAuth(publicCfg)
	if err != nil {
		return nil, err
	}
	privateAuth, err := auth.RequireInternalAPIKey(privateCfg)
	if err != nil {
		return nil, err
	}

	// Using default setting of ow-common-mods auth middleware
	return &TopologyAuth{
		publicAuth:  publicAuth,
		privateAuth: privateAuth,
	}, nil
}

func (ta *TopologyAuth) GetPublicAuthHandler() fiber.Handler {
	return ta.publicAuth
}

func (ta *TopologyAuth) GetPrivateAuthHandler() fiber.Handler {
	return ta.privateAuth
}
