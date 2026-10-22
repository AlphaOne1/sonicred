// SPDX-FileCopyrightText: 2026 The SonicRed contributors.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/acme"
)

const OSWindows = "windows"

func generateCertAndKey() (string, string) {
	// generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	// certificate information
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Example Organization"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // validity: 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// generate self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		panic(err)
	}

	// save certificate
	certFile, err := os.CreateTemp("", "cert")
	if err != nil {
		panic(err)
	}

	defer func() { _ = certFile.Close() }()

	_ = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// save private key
	keyFile, err := os.CreateTemp("", "key")
	if err != nil {
		panic(err)
	}

	defer func() { _ = keyFile.Close() }()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		panic(err)
	}

	_ = pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return certFile.Name(), keyFile.Name()
}

type testHelper interface {
	Helper()
	Errorf(format string, a ...any)
}

func sendMe(helper testHelper, sig os.Signal) {
	helper.Helper()

	currentPID := os.Getpid()
	currentProcess, currentProcessErr := os.FindProcess(currentPID)

	if currentProcessErr != nil {
		helper.Errorf("could not get process id")
		return
	}

	switch runtime.GOOS {
	case OSWindows:
		helper.Errorf("Go does not yet support anything else than KILL on Windows")
		// "golang.org/x/sys/windows"
		// if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_C_EVENT, 0); err != nil {
		// 	helper.Errorf("could not send signal %s: %v", sig.String(), err)
		// }
		sig = os.Kill

		fallthrough
	default:
		if signalErr := currentProcess.Signal(sig); signalErr != nil {
			helper.Errorf("could not send signal %s: %v", sig.String(), signalErr)
		}
	}
}

func startMain(helper testHelper, args ...string) (*time.Timer, chan int) {
	helper.Helper()
	// exitFunc replaces os.Exit with this function that will end main, and we can catch the error here
	exitFunc = func(code int) {
		slog.Info("main called exit", slog.Int("code", code))
		panic(code)
	}

	buildInfoTag = "test"

	// mainResult will hold the result of main
	mainReturn := make(chan int, 1)
	mainStart := make(chan struct{}, 1)

	slog.Info("starting main")

	go func() {
		os.Args = args
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		defer func() {
			if r := recover(); r != nil {
				if msg, msgConv := r.(int); msgConv {
					mainReturn <- msg
				}
			}
		}()

		mainStart <- struct{}{}

		main()

		// we can just come here if main did not call anyhow the exit function
		// normal returns from main signalize no error --> 0
		mainReturn <- 0
	}()

	<-mainStart

	slog.Info("setting exit timeout")

	afterTimer := time.NewTimer(2 * time.Second)

	return afterTimer, mainReturn
}

func finalizeMain(helper testHelper, afterTimer *time.Timer, result chan int) int {
	helper.Helper()

	select {
	case r := <-result:
		if !afterTimer.Stop() {
			select {
			case <-afterTimer.C:
			default:
			}
		}

		return r
	case <-afterTimer.C:
		slog.Info("exit timer timeout")
		sendMe(helper, syscall.SIGTERM)
	}

	return <-result
}

func TestSonicMain(t *testing.T) {
	if runtime.GOOS == OSWindows {
		t.Skip("windows not supported yet")
	}

	afterTimer, mainReturn := startMain(t,
		"sonicred",
		"-root", "./testroot",
		"-header", "X-Test-Header: testHeaderContent",
		"-header", "X-Empty",
		"-headerfile", "testroot/testHeaders.conf",
		"-tryfile", "$uri",
		"-tryfile", "/index.html",
		"-address", "localhost",
		"-iaddress", "localhost",
	)

	couldRequest := false

	for i := 0; i < 10 && !couldRequest; i++ {
		req, _ := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"http://localhost:8080/index.html",
			nil)
		res, err := http.DefaultClient.Do(req)

		if err != nil {
			runtime.Gosched()
			slog.Info("received error", slog.String("error", err.Error()))
			time.Sleep(500 * time.Millisecond)

			continue
		}

		defer func() {
			if err := res.Body.Close(); err != nil {
				t.Errorf("could not close response: %v", err)
			}
		}()

		couldRequest = true

		assert.Equal(t, http.StatusOK, res.StatusCode, "status code should be 200")
		assert.Equal(t,
			"testHeaderContent",
			res.Header.Get("X-Test-Header"),
			"header should contain X-Test-Header with testHeaderContent")
		assert.Contains(t,
			res.Header,
			"X-Empty",
			"X-Empty header not found")
		assert.Equal(t,
			"line0 line1",
			res.Header.Get("X-File-Test-0"),
			"header should contain X-File-Test-0")
		assert.Equal(t,
			"line2",
			res.Header.Get("X-File-Test-1"),
			"header should contain X-File-Test-1")
	}

	assert.True(t, couldRequest, "could not send any request")

	result := finalizeMain(t, afterTimer, mainReturn)

	slog.Info("main returned", slog.Int("result", result))
}

func TestSonicMainTLS(t *testing.T) {
	if runtime.GOOS == OSWindows {
		t.Skip("windows not supported yet")
	}

	certFile, keyFile := generateCertAndKey()

	afterTimer, mainReturn := startMain(t,
		"sonicred",
		"-root", "./testroot",
		"-tlscert", certFile,
		"-tlskey", keyFile,
		"-header", "X-Test-Header: testHeaderContent",
		"-header", "X-Empty",
		"-headerfile", "testroot/testHeaders.conf",
		"-tryfile", "$uri",
		"-tryfile", "/index.html",
		"-address", "localhost",
		"-iaddress", "localhost",
	)

	couldRequest := false

	for i := 0; i < 10 && !couldRequest; i++ {
		client := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}

		req, _ := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"https://localhost:8080/index.html",
			nil)
		res, err := client.Do(req)

		if err != nil {
			runtime.Gosched()
			t.Logf("received error: %v\n", err)
			time.Sleep(500 * time.Millisecond)

			continue
		}

		defer func() {
			if err := res.Body.Close(); err != nil {
				t.Errorf("could not close response: %v", err)
			}
		}()

		couldRequest = true

		assert.Equal(t, http.StatusOK, res.StatusCode, "status code should be 200")
		assert.Equal(t,
			"testHeaderContent",
			res.Header.Get("X-Test-Header"),
			"header should contain X-Test-Header with testHeaderContent")
		assert.Contains(t,
			res.Header,
			"X-Empty",
			"X-Empty header not found")
		assert.Equal(t,
			"line0 line1",
			res.Header.Get("X-File-Test-0"),
			"header should contain X-File-Test-0")
		assert.Equal(t,
			"line2",
			res.Header.Get("X-File-Test-1"),
			"header should contain X-File-Test-1")
	}

	assert.True(t, couldRequest, "could not send any request")

	result := finalizeMain(t, afterTimer, mainReturn)

	slog.Info("main returned", slog.Int("result", result))

	_ = os.Remove(certFile)
	_ = os.Remove(keyFile)
}

func TestSonicMainVersion(t *testing.T) {
	afterTimer, mainReturn := startMain(t,
		"sonicred", "-version",
	)

	runtime.Gosched()

	result := finalizeMain(t, afterTimer, mainReturn)

	assert.Equal(t, 0, result, "expected successful return")
}

func TestSonicMainInvalidRoot(t *testing.T) {
	afterTimer, mainReturn := startMain(t,
		"sonicred",
		"-root", "/noentry",
		"-address", "localhost",
		"-iaddress", "localhost",
	)

	runtime.Gosched()

	result := finalizeMain(t, afterTimer, mainReturn)

	assert.Equal(t, 1, result, "main should exit with 1")
}

func TestSonicMainInvalidHeaderFile(t *testing.T) {
	afterTimer, mainReturn := startMain(t,
		"sonicred",
		"-root", "testroot/",
		"-headerfile", "/noexist",
		"-address", "localhost",
		"-iaddress", "localhost",
	)

	runtime.Gosched()

	result := finalizeMain(t, afterTimer, mainReturn)

	assert.Equal(t, 1, result, "main should exit with 1")
}

func TestSonicMainInvalidWAFFile(t *testing.T) {
	afterTimer, mainReturn := startMain(t,
		"sonicred",
		"-root", "testroot/",
		"-wafcfg", "/noexist",
		"-address", "localhost",
		"-iaddress", "localhost",
	)

	runtime.Gosched()

	result := finalizeMain(t, afterTimer, mainReturn)

	assert.Equal(t, 1, result, "main should exit with 1")
}

func TestConfigureClientCAs(t *testing.T) {
	t.Parallel()

	validCA, validKey := generateCertAndKey()

	t.Cleanup(func() {
		_ = os.Remove(validCA)
		_ = os.Remove(validKey)
	})

	invalidCA := t.TempDir() + "/invalid.pem"

	if err := os.WriteFile(invalidCA, []byte("no certificate"), 0o600); err != nil {
		t.Fatalf("could not write invalid CA file: %v", err)
	}

	tests := []struct {
		files   []string
		wantErr error
	}{
		{files: []string{validCA}, wantErr: nil},
		{files: []string{validCA, invalidCA}, wantErr: errTLSConfig},
		{files: []string{invalidCA}, wantErr: errTLSConfig},
	}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("TestConfigureClientCAs-%d", testNum), func(t *testing.T) {
			t.Parallel()

			err := configureClientCAs(&tls.Config{MinVersion: tls.VersionTLS13}, test.files)

			if test.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, test.wantErr)
			}
		})
	}
}

func TestTLSMinVersion(t *testing.T) {
	t.Parallel()

	cert, key := generateCertAndKey()

	t.Cleanup(func() {
		_ = os.Remove(cert)
		_ = os.Remove(key)
	})

	certConfig, certErr := generateTLSConfig(cert, key, nil, "", "", nil)
	require.NoError(t, certErr)
	assert.Equal(t, uint16(tls.VersionTLS13), certConfig.MinVersion, "certificate config")

	acmeConfig, acmeErr := generateTLSConfig("", "", []string{"example.com"}, t.TempDir(), "", nil)
	require.NoError(t, acmeErr)
	assert.Equal(t, uint16(tls.VersionTLS13), acmeConfig.MinVersion, "acme config")
}

// generateSelfSignedCert generates a self-signed certificate with the given extended key usage.
func generateSelfSignedCert(t *testing.T, usage x509.ExtKeyUsage) (tls.Certificate, *x509.Certificate) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "could not generate key")

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		DNSNames:              []string{"localhost"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{usage},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err, "could not create certificate")

	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err, "could not parse certificate")

	return tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: priv, Leaf: cert}, cert
}

func TestACMEChallengeWithClientCAs(t *testing.T) {
	t.Parallel()

	const protoHTTP1 = "http/1.1"

	serverCert, _ := generateSelfSignedCert(t, x509.ExtKeyUsageServerAuth)
	clientCert, clientCA := generateSelfSignedCert(t, x509.ExtKeyUsageClientAuth)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(clientCA)

	// mimics the configuration of autocert with client certificate authentication
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		NextProtos:   []string{protoHTTP1, acme.ALPNProto},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}

	allowACMEChallenges(config)

	// with TLS 1.3, the client considers the handshake complete before the server checks the client certificate,
	// so the configuration chosen for the connections is checked directly
	require.NotNil(t, config.GetConfigForClient)

	challengeConfig, err := config.GetConfigForClient(&tls.ClientHelloInfo{SupportedProtos: []string{acme.ALPNProto}})
	require.NoError(t, err)
	require.NotNil(t, challengeConfig, "challenge connections must use the challenge config")
	assert.Equal(t, tls.NoClientCert, challengeConfig.ClientAuth)

	regularConfig, err := config.GetConfigForClient(
		&tls.ClientHelloInfo{SupportedProtos: []string{protoHTTP1, acme.ALPNProto}})
	require.NoError(t, err)
	assert.Nil(t, regularConfig, "regular connections must use the original config")

	listener, err := tls.Listen("tcp", "127.0.0.1:0", config)
	require.NoError(t, err)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "content")
		}),
		ReadHeaderTimeout: ReadTimeout,
	}

	go func() { _ = server.Serve(listener) }()

	t.Cleanup(func() { _ = server.Close() })

	// dial connects to the server, with TLS 1.3 the client side handshake succeeds before the server checks the
	// client certificate
	dial := func(protos []string, certs []tls.Certificate) *tls.Conn {
		dialer := tls.Dialer{Config: &tls.Config{
			NextProtos:         protos,
			Certificates:       certs,
			InsecureSkipVerify: true, // self-signed test certificate
			MinVersion:         tls.VersionTLS13,
		}}

		conn, err := dialer.DialContext(t.Context(), "tcp", listener.Addr().String())
		require.NoError(t, err)

		t.Cleanup(func() { _ = conn.Close() })

		return conn.(*tls.Conn) //nolint:forcetypeassert // tls.Dialer always returns a *tls.Conn
	}

	// get sends a GET request on the connection and returns the status code of the response
	get := func(conn *tls.Conn) (int, error) {
		_, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
		require.NoError(t, err)

		response, err := http.ReadResponse(bufio.NewReader(conn), nil)

		if err != nil {
			return 0, err //nolint:wrapcheck // only used in this test
		}

		_ = response.Body.Close()

		return response.StatusCode, nil
	}

	// regular connection without client certificate is rejected
	_, err = get(dial([]string{protoHTTP1}, nil))
	require.Error(t, err, "connection without client certificate must be rejected")

	// regular connection with client certificate is served
	status, err := get(dial([]string{protoHTTP1}, []tls.Certificate{clientCert}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)

	// challenge connection completes the handshake without client certificate, but serves no content
	conn := dial([]string{acme.ALPNProto}, nil)
	assert.Equal(t, acme.ALPNProto, conn.ConnectionState().NegotiatedProtocol)

	_, err = get(conn)
	require.Error(t, err, "challenge connections must not serve content")
}

func TestPrepareCertCache(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == OSWindows {
		t.Skip("symlinks and permission bits not supported on windows")
	}

	rootPath := t.TempDir()
	outside := t.TempDir()

	require.NoError(t, os.Symlink(rootPath, outside+"/linkToRoot"))
	require.NoError(t, os.Mkdir(outside+"/shared", 0o755))

	tests := []struct {
		certCache string
		wantErr   error
	}{
		{certCache: outside + "/new/cache"},
		{certCache: outside + "/shared"}, // only a warning for user-given directories
		{certCache: rootPath, wantErr: errCertCache},
		{certCache: rootPath + "/cache", wantErr: errCertCache},
		{certCache: outside + "/linkToRoot/cache", wantErr: errCertCache},
	}

	for testNum, test := range tests {
		t.Run(fmt.Sprintf("TestPrepareCertCache-%d", testNum), func(t *testing.T) {
			t.Parallel()

			certCache, err := prepareCertCache(test.certCache, rootPath)

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.certCache, certCache)
			assert.DirExists(t, certCache)
		})
	}
}

func TestPrepareCertCacheSymlinkedRoot(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == OSWindows {
		t.Skip("symlinks not supported on windows")
	}

	realRoot := t.TempDir()
	rootPath := t.TempDir() + "/root"

	require.NoError(t, os.Symlink(realRoot, rootPath))

	_, err := prepareCertCache(rootPath+"/cache", rootPath)
	require.ErrorIs(t, err, errCertCache)

	// the rejected directory must not have been created inside of the web root directory
	assert.NoDirExists(t, realRoot+"/cache")
}

// TestPrepareCertCacheDefault is not parallel, as it modifies the environment.
func TestPrepareCertCacheDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CACHE_HOME is only used on linux")
	}

	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	certCache, err := prepareCertCache("", t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, cacheHome+"/sonicred/autocert", certCache)

	info, err := os.Stat(certCache)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(certCacheMode), info.Mode().Perm())

	// the default directory must not be accessible by others
	require.NoError(t, os.Chmod(certCache, 0o755))

	_, err = prepareCertCache("", t.TempDir())
	require.ErrorIs(t, err, errCertCache)
}

func BenchmarkHandler(b *testing.B) {
	fileHandler, fileCleanup, fileHandlerErr := generateFileHandler(
		false,
		"/",
		"testroot/",
		true,
		nil,
		nil,
		nil)

	if fileHandlerErr != nil {
		b.Fatalf("could not generate file handlers: %v", fileHandlerErr)
	}

	defer fileCleanup()

	server := httptest.NewServer(fileHandler)

	defer server.Close()

	client := &http.Client{}

	for b.Loop() {
		req, _ := http.NewRequestWithContext(
			b.Context(),
			http.MethodGet,
			server.URL,
			nil)
		resp, err := client.Do(req)

		if err != nil {
			b.Fatalf("Failed to make GET request: %v", err)
		}

		_, err = io.Copy(io.Discard, resp.Body)

		if err != nil {
			b.Fatalf("Failed to read response body: %v", err)
		}

		_ = resp.Body.Close()
	}
}

func sonicMainHandlerTest(t *testing.T, uri string, method string, header string, headerValue string) {
	t.Helper()

	fileHandler, fileCleanup, fileHandlerErr := generateFileHandler(
		false,
		"/",
		"testroot/",
		true,
		nil,
		nil,
		nil)

	if fileHandlerErr != nil {
		t.Fatalf("could not generate file handlers: %v", fileHandlerErr)
	}

	defer fileCleanup()

	// Basic input constraints to keep fuzzing focused and avoid trivial rejections
	if len(uri) == 0 || len(uri) > 1024 {
		t.Skip()
	}

	// Ensure uri is a path; strip spaces and reject NUL
	uri = strings.TrimSpace(uri)

	if strings.ContainsRune(uri, '\x00') {
		t.Skip()
	}

	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}

	urlParts := strings.Split(uri, "/")

	for i := range urlParts {
		urlParts[i] = url.PathEscape(urlParts[i])
	}

	uri = strings.Join(urlParts, "/")

	if _, err := url.Parse(uri); err != nil {
		t.Skip()
	}

	// Only fuzz a small, meaningful method set to reduce noise
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		method = strings.ToUpper(method)
	default:
		t.Skip()
	}

	// Validate header name; only set it if it looks sane
	validHeader := func(s string) bool {
		if s == "" || len(s) > 128 {
			return false
		}

		for i := range len(s) {
			c := s[i]
			isAlphaNum :=
				(c >= 'a' && c <= 'z') ||
					(c >= 'A' && c <= 'Z') ||
					(c >= '0' && c <= '9')

			if !isAlphaNum && c != '-' {
				return false
			}
		}

		return true
	}

	if len(headerValue) > 1024 {
		// Avoid huge header values that only stress http parsing without adding coverage
		t.Skip()
	}

	rec := httptest.NewRecorder()

	req := httptest.NewRequestWithContext(
		t.Context(),
		method,
		"http://localhost:8080"+uri,
		nil)

	if validHeader(header) {
		req.Header.Set(header, headerValue)
	}

	fileHandler.ServeHTTP(rec, req)

	if rec.Result().StatusCode >= http.StatusInternalServerError {
		t.Errorf("received status code %s (%d) for %s %s",
			http.StatusText(rec.Result().StatusCode),
			rec.Result().StatusCode,
			method,
			uri)
	}

	if err := rec.Result().Body.Close(); err != nil {
		t.Errorf("could not close response: %v", err)
	}
}

func FuzzSonicMain(f *testing.F) {
	f.Add("index.html", "GET", "X-Fuzz", "value")
	f.Add("index.html", "HEAD", "X-Empty", "")
	f.Add("%", "GET", "0", "0")
	f.Add("\xd8", "HEAD", "0", "")
	f.Add("0 0", "GET", "0", "0")
	f.Add(strings.Repeat("0", 256), "HEAD", "0", "0")

	f.Fuzz(sonicMainHandlerTest)
}

func TestSonicMainHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		uri         string
		method      string
		header      string
		headerValue string
	}{
		{uri: "%", method: "GET", header: "0", headerValue: "0"},
		{uri: "\xd8", method: "HEAD", header: "0", headerValue: ""},
		{uri: "0 0", method: "GET", header: "0", headerValue: "0"},
		{uri: strings.Repeat("0", 256), method: "HEAD", header: "0", headerValue: "0"},
	}

	for testIndex, test := range tests {
		t.Run(fmt.Sprintf("TestSonicMainHandler-%d", testIndex), func(t *testing.T) {
			t.Parallel()

			sonicMainHandlerTest(t,
				test.uri,
				test.method,
				test.header,
				test.headerValue,
			)
		})
	}
}
