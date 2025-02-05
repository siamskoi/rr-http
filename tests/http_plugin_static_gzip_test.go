package tests

import (
	"fmt"
	"github.com/roadrunner-server/config/v5"
	"github.com/roadrunner-server/endure/v2"
	httpPlugin "github.com/roadrunner-server/http/v5"
	"github.com/roadrunner-server/logger/v5"
	"github.com/roadrunner-server/server/v5"
	"github.com/roadrunner-server/static/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestStaticGzipEnabled(t *testing.T) {
	runStaticGzipTest(t, "configs/.rr-http-static-gzip.yaml", true)
}

func TestStaticGzipDisabled(t *testing.T) {
	runStaticGzipTest(t, "configs/.rr-http-static.yaml", false)
}

func runStaticGzipTest(t *testing.T, configPath string, serverGzipEnabled bool) {
	cfg := &config.Plugin{Version: "2023.3.5", Path: configPath}
	wg, stopCh := setupStaticGzipTest(t, cfg)

	time.Sleep(time.Second) // Небольшая задержка

	if serverGzipEnabled {
		t.Run("ServeEnableGzipOnClient", createClientTest(21603, "sample-big.txt", false, true))
		t.Run("ServeDisableGzipOnClient", createClientTest(21603, "sample-big.txt", true, false))
	} else {
		t.Run("ServeEnableGzipOnClient", createClientTest(21603, "sample-big.txt", false, false))
		t.Run("ServeDisableGzipOnClient", createClientTest(21603, "sample-big.txt", true, false))
	}

	stopCh <- struct{}{}
	wg.Wait()
}

func setupStaticGzipTest(t *testing.T, cfg *config.Plugin) (*sync.WaitGroup, chan struct{}) {
	cont := endure.New(slog.LevelDebug)

	assert.NoError(t, cont.RegisterAll(cfg, &logger.Plugin{}, &server.Plugin{}, &httpPlugin.Plugin{}, &static.Plugin{}))
	require.NoError(t, cont.Init())

	ch, err := cont.Serve()
	require.NoError(t, err)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	stopCh := make(chan struct{}, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case e := <-ch:
				assert.Fail(t, "error", e.Error.Error())
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
			case <-sig:
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			case <-stopCh:
				// timeout
				err = cont.Stop()
				if err != nil {
					assert.FailNow(t, "error", err.Error())
				}
				return
			}
		}
	}()

	return wg, stopCh
}

func getTransport(disableCompression bool) *http.Transport {
	return &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: disableCompression,
	}
}

func createClientTest(port int, filename string, disableCompression, expectUncompressed bool) func(t *testing.T) {
	return func(t *testing.T) {
		client := &http.Client{Transport: getTransport(disableCompression)}

		url := fmt.Sprintf("http://127.0.0.1:%d/%s", port, filename)
		resp, err := client.Get(url)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		require.Equal(t, expectUncompressed, resp.Uncompressed)
		require.Contains(t, string(body), "sample")
	}
}
