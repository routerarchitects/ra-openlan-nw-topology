package common

import (
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3/client"
)

const (
	DefaultRequestTimeout = 15 * time.Second
	defaultInternalName   = "nw-topology-service"
)

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

func NormalizeTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	return DefaultRequestTimeout
}

func NormalizeInternalName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultInternalName
	}
	return name
}
