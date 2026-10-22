// SPDX-FileCopyrightText: 2026 The SonicRed contributors.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var errTLSConfig = errors.New("invalid tls configuration")

// errCertCache indicates an unusable certificate cache directory.
var errCertCache = errors.New("invalid certificate cache")

// certCacheMode is the permission used to create the certificate cache directory, as it contains private keys.
const certCacheMode = 0o700

// generateTLSConfig generates a new TLS configuration if the parameters are set accordingly.
// To use a user-supplied cert- and key file, only specify those two parameters. Specifying
// the acmeDomains will lead to an error in this case.
// To use the Let's Encrypt feature, cert and key are to be left empty and acmeDomains must
// be specified.
// If nothing is specified, no TLS configuration is generated.
func generateTLSConfig(
	cert string,
	key string,
	acmeDomains []string,
	certCache string,
	acmeEndpoint string,
	clientCAs []string) (*tls.Config, error) {

	if err := validateTLSParams(cert, key, acmeDomains, clientCAs); err != nil {
		return nil, err
	}

	// completely valid, we do not have a TLS configuration
	if len(cert) == 0 && len(acmeDomains) == 0 {
		return nil, nil
	}

	var config *tls.Config
	var err error

	if len(cert) > 0 {
		config, err = createCertificateConfig(cert, key)

		if err != nil {
			return nil, err
		}
	}

	if len(acmeDomains) > 0 {
		config = createACMEConfig(acmeDomains, certCache, acmeEndpoint)
	}

	if config != nil && len(clientCAs) > 0 {
		if err := configureClientCAs(config, clientCAs); err != nil {
			return nil, err
		}

		if len(acmeDomains) > 0 {
			allowACMEChallenges(config)
		}
	}

	return config, nil
}

// validateTLSParams validates the provided TLS configuration parameters according to specific constraints.
// Ensures cert and key are both set or unset, acmeDomains mutually exclude cert/key, and clientCAs require TLS setup.
// Returns an error if parameters are invalid, otherwise nil.
func validateTLSParams(cert, key string, acmeDomains, clientCAs []string) error {
	if (len(cert) > 0) != (len(key) > 0) {
		return fmt.Errorf("cert and key must both be given or not given: %w", errTLSConfig)
	}

	if len(cert) > 0 && len(acmeDomains) > 0 {
		return fmt.Errorf("either cert+key or acmeDomains are to be given: %w", errTLSConfig)
	}

	if len(cert) == 0 && len(acmeDomains) == 0 && len(clientCAs) > 0 {
		return fmt.Errorf("clientCAs are only valid if cert+key or acmeDomains are given: %w", errTLSConfig)
	}

	return nil
}

// createCertificateConfig loads a TLS certificate and private key and returns a configured TLS configuration.
// certFile is the path to the certificate file. keyFile is the path to the private key file.
// Returns a tls.Config instance on success or an error if loading the certificate or key fails.
func createCertificateConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)

	if err != nil {
		return nil, fmt.Errorf("could not load certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// createACMEConfig initializes and returns a TLS configuration for handling ACME-based certificate management.
// acmeDomains specifies the list of allowed domains for certificate provisioning.
// certCache defines the file path where certificates are cached.
// acmeEndpoint is the optional URL for a custom ACME directory endpoint. If empty, the default endpoint is used.
func createACMEConfig(acmeDomains []string, certCache, acmeEndpoint string) *tls.Config {
	var acmeClient *acme.Client

	if len(acmeEndpoint) > 0 {
		acmeClient = &acme.Client{
			DirectoryURL: acmeEndpoint,
		}
	}

	certManager := autocert.Manager{
		Cache:      autocert.DirCache(certCache),
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(acmeDomains...),
		Client:     acmeClient,
	}

	config := certManager.TLSConfig()
	config.MinVersion = tls.VersionTLS13

	return config
}

// configureClientCAs sets up the ClientCA pool in the provided tls.Config using the
// provided list of CA file paths. It reads each file, appends its certificates to a
// new cert pool, and configures the config for client certificate auth.
func configureClientCAs(config *tls.Config, clientCAs []string) error {
	clientCAPool := x509.NewCertPool()

	for _, ca := range clientCAs {
		caFile, err := os.ReadFile(filepath.Clean(ca))

		if err != nil {
			return fmt.Errorf("could not read client CA file: %w", err)
		}

		if !clientCAPool.AppendCertsFromPEM(caFile) {
			return fmt.Errorf("no valid PEM certificate found in client CA file %v: %w", ca, errTLSConfig)
		}
	}

	config.ClientCAs = clientCAPool
	config.ClientAuth = tls.RequireAndVerifyClientCert

	return nil
}

// isACMEChallenge reports if the client hello belongs to a TLS-ALPN-01 challenge connection of the ACME server.
// It uses the same criteria as autocert to decide if the challenge certificate is to be presented.
func isACMEChallenge(hello *tls.ClientHelloInfo) bool {
	return len(hello.SupportedProtos) == 1 && hello.SupportedProtos[0] == acme.ALPNProto
}

// allowACMEChallenges lets the TLS-ALPN-01 challenge connections of the ACME server pass without a client
// certificate, as the ACME server cannot provide one. All other connections still require a valid client certificate.
// The challenge connections cannot be used to access the served content, as the HTTP server closes connections with
// the unknown protocol acme-tls/1 right after the handshake.
func allowACMEChallenges(config *tls.Config) {
	challengeConfig := config.Clone()
	challengeConfig.ClientAuth = tls.NoClientCert
	challengeConfig.ClientCAs = nil
	challengeConfig.NextProtos = []string{acme.ALPNProto}

	config.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		if isACMEChallenge(hello) {
			return challengeConfig, nil
		}

		// a nil config means that the original config is used
		return nil, nil
	}
}

// defaultCertCache returns the default certificate cache directory inside the user cache directory. If the user
// cache directory cannot be determined, a directory in the temporary directory is used instead.
func defaultCertCache() string {
	if cacheDir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(cacheDir, "sonicred", "autocert")
	}

	fallback := filepath.Join(os.TempDir(), "sonicred-autocert")

	slog.Warn("could not determine user cache directory, using temporary directory for certificate cache",
		slog.String("path", fallback))

	return fallback
}

// isInside reports if path is the directory dir or located inside of it.
func isInside(path, dir string) bool {
	absPath, pathErr := filepath.Abs(path)
	absDir, dirErr := filepath.Abs(dir)

	if pathErr != nil || dirErr != nil {
		// cannot be decided, assume the worst
		return true
	}

	rel, err := filepath.Rel(absDir, absPath)

	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// prepareCertCache determines the certificate cache directory, using the default if certCache is empty, and creates
// it. As the cache contains private keys, it must not be located inside the web root directory. The default
// directory must also be only accessible by the current user, for user-given directories this only produces a warning.
func prepareCertCache(certCache, rootPath string) (string, error) {
	isDefault := certCache == ""

	if isDefault {
		certCache = defaultCertCache()
	}

	resolvedRoot, rootErr := filepath.EvalSymlinks(rootPath)

	if rootErr != nil {
		return "", fmt.Errorf("could not resolve web root directory: %w", rootErr)
	}

	insideRootErr := fmt.Errorf("certificate cache %v must not be inside of the web root directory %v: %w",
		certCache, rootPath, errCertCache)

	// checked before creating, to not create directories inside of the web root directory, the unresolved root
	// directory is also checked, as the certificate cache path might contain it as a symbolic link
	if isInside(certCache, rootPath) || isInside(certCache, resolvedRoot) {
		return "", insideRootErr
	}

	if err := os.MkdirAll(certCache, certCacheMode); err != nil {
		return "", fmt.Errorf("could not create certificate cache directory: %w", err)
	}

	resolvedCache, cacheErr := filepath.EvalSymlinks(certCache)

	if cacheErr != nil {
		return "", fmt.Errorf("could not resolve certificate cache directory: %w", cacheErr)
	}

	// checked again after resolving symbolic links in the certificate cache path
	if isInside(resolvedCache, resolvedRoot) {
		return "", insideRootErr
	}

	info, err := os.Stat(certCache)

	if err != nil {
		return "", fmt.Errorf("could not get info of certificate cache directory: %w", err)
	}

	if problem := certCacheAccessProblem(info); problem != "" {
		if isDefault {
			return "", fmt.Errorf("certificate cache %v %s: %w", certCache, problem, errCertCache)
		}

		slog.Warn("certificate cache "+problem, slog.String("path", certCache))
	}

	slog.Info("using certificate cache", slog.String("path", certCache))

	return certCache, nil
}
