package common

import (
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/client"
	servicediscovery "github.com/routerarchitects/ow-common-mods/servicediscovery"
)

type ServiceRPCBase struct {
	Discovery    *servicediscovery.Discovery
	Client       *client.Client
	Timeout      time.Duration
	InternalName string
	Logger       *slog.Logger
}

func NewServiceRPCBase(
	discovery *servicediscovery.Discovery,
	tlsRootCA string,
	timeout time.Duration,
	internalName string,
	Logger *slog.Logger,
) *ServiceRPCBase {
	return &ServiceRPCBase{
		Discovery:    discovery,
		Client:       NewFiberClient(timeout, tlsRootCA),
		Timeout:      timeout,
		InternalName: internalName,
		Logger:       Logger,
	}
}

func NewFiberClient(timeout time.Duration, tlsRootCA string) *client.Client {
	fiberClient := client.New()
	fiberClient.SetTimeout(timeout)

	if strings.TrimSpace(tlsRootCA) == "" {
		return fiberClient
	}

	pemBytes, err := os.ReadFile(tlsRootCA)
	if err != nil {
		panic(fmt.Sprintf("read TLS root CA %q: %v", tlsRootCA, err))
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		panic(fmt.Sprintf("parse TLS root CA %q: invalid PEM", tlsRootCA))
	}

	fiberClient.TLSConfig().RootCAs = pool
	return fiberClient
}
