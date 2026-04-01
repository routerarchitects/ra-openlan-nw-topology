package services

import (
	"crypto/x509"
	"encoding/pem"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	// "github.com/gofiber/fiber"
	// "github.com/gofiber/fiber"
	"github.com/router-architects/ra-openlan-nw-topology/adapters/apperrors"
	logger "github.com/router-architects/ra-openlan-nw-topology/adapters/logger"
	"github.com/router-architects/ra-openlan-nw-topology/internal/models"
	"github.com/sirupsen/logrus"
)

func ListSubsystemLogLevelPairs() []logger.SubsystemLogLevel {
	return logger.ListSubsystemLogLevelPairs()
}

func SetSubsystemLevel(payload *models.SetSubsytemPayload) {
	for _, s := range payload.Subsystems {
		level, err := logrus.ParseLevel(s.Value)
		if err != nil {
			continue // skip invalid level
		}
		logger.SetSubsystemLevel(s.Tag, level)
	}
}

func ListSubsystemNames() []string {
	return logger.ListSubsystemNames()
}

func GetLogLevelNames() []string {
	return []string{"trace", "debug", "info", "warn", "error", "fatal"}
}

func SystemInfo() map[string]interface{} {
	// 1. System info
	hostname, _ := os.Hostname()

	// 2. Processor count
	procs := runtime.NumCPU()

	// 3. OS
	osname := runtime.GOOS

	// 4. UI URL (add to config or env)
	uiURL := os.Getenv("UI_URL")
	if uiURL == "" {
		uiURL = "http://localhost"
	}

	// 5. Certificates directory (configurable)
	certDir := "./certs" // change to real cert folder
	certList := []map[string]interface{}{}

	filepath.WalkDir(certDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".pem") {
			filename, expires, err := loadCertificateInfo(path)
			if err == nil {
				certList = append(certList, map[string]interface{}{
					"filename":  filename,
					"expiresOn": expires,
				})
			}
		}
		return nil
	})

	// 6. Build Response
	return map[string]interface{}{
		"UI":           uiURL,
		"certificates": certList,
		"hostname":     hostname,
		"os":           osname,
		"processors":   procs,
		"start":        getStartTime(),
		"uptime":       getUptimeSeconds(),
		"version":      "3.2.0(1) - e777a8d", // replace with config or build variable
	}
}

var programStart = time.Now()

func loadCertificateInfo(certPath string) (filename string, expires int64, err error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", 0, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return "", 0, apperrors.WrapError(apperrors.CodeInternal, "invalid PEM block", nil)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", 0, err
	}

	return filepath.Base(certPath), cert.NotAfter.Unix(), nil
}

func getStartTime() int64 {
	return programStart.Unix()
}

func getUptimeSeconds() int64 {
	return int64(time.Since(programStart).Seconds())
}
