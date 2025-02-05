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
	cfg := &config.Plugin{
		Version: "2023.3.5",
		Path:    "configs/.rr-http-static-gzip.yaml",
	}

	wg, stopCh := testStaticGzip(t, cfg)

	time.Sleep(time.Second)

	t.Run("ServeEnableGzipOnClient", enableGzipOnClient(21603, "sample-big.txt"))
	t.Run("ServeDisableGzipOnClient", disableGzipOnClient(21603, "sample-big.txt"))

	stopCh <- struct{}{}
	wg.Wait()
}

func TestStaticGzipDisabled(t *testing.T) {
	cfg := &config.Plugin{
		Version: "2023.3.5",
		Path:    "configs/.rr-http-static.yaml",
	}

	wg, stopCh := testStaticGzip(t, cfg)

	time.Sleep(time.Second)

	t.Run("ServeEnableGzipOnClient", enableGzipOnClientAndDisableOnServe(21603, "sample-big.txt"))
	t.Run("ServeDisableGzipOnClient", disableGzipOnClientAndDisableOnServer(21603, "sample-big.txt"))

	stopCh <- struct{}{}
	wg.Wait()
}

func testStaticGzip(t *testing.T, cfg *config.Plugin) (*sync.WaitGroup, chan struct{}) {
	cont := endure.New(slog.LevelDebug)

	err := cont.RegisterAll(
		cfg,
		&logger.Plugin{},
		&server.Plugin{},
		&httpPlugin.Plugin{},
		&static.Plugin{},
	)
	assert.NoError(t, err)

	err = cont.Init()
	if err != nil {
		t.Fatal(err)
	}

	ch, err := cont.Serve()
	assert.NoError(t, err)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	wg.Add(1)

	stopCh := make(chan struct{}, 1)

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

func enableGzipOnClient(port int, filename string) func(t *testing.T) {
	return func(t *testing.T) {
		tr := &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		}
		client := &http.Client{Transport: tr}
		url := fmt.Sprintf("http://127.0.0.1:%d/%s", port, filename)
		r, err := client.Get(url)
		require.NoError(t, err)
		assert.Equal(t, 200, r.StatusCode)
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, true, r.Uncompressed)
		require.Contains(t, string(b), "sample")
		_ = r.Body.Close()
	}
}

func disableGzipOnClient(port int, filename string) func(t *testing.T) {
	return func(t *testing.T) {
		tr := &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		}
		client := &http.Client{Transport: tr}
		url := fmt.Sprintf("http://127.0.0.1:%d/%s", port, filename)
		r, err := client.Get(url)
		require.NoError(t, err)
		assert.Equal(t, 200, r.StatusCode)

		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		require.Equal(t, false, r.Uncompressed)

		require.Contains(t, string(b), "sample")
		_ = r.Body.Close()
	}
}

func disableGzipOnClientAndDisableOnServer(port int, filename string) func(t *testing.T) {
	return func(t *testing.T) {
		tr := &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		}
		client := &http.Client{Transport: tr}
		url := fmt.Sprintf("http://127.0.0.1:%d/%s", port, filename)
		r, err := client.Get(url)
		require.NoError(t, err)
		assert.Equal(t, 200, r.StatusCode)

		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		require.Equal(t, false, r.Uncompressed)

		require.Contains(t, string(b), "sample")
		_ = r.Body.Close()
	}
}

func enableGzipOnClientAndDisableOnServe(port int, filename string) func(t *testing.T) {
	return func(t *testing.T) {
		tr := &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: false,
		}
		client := &http.Client{Transport: tr}
		url := fmt.Sprintf("http://127.0.0.1:%d/%s", port, filename)
		r, err := client.Get(url)
		require.NoError(t, err)
		assert.Equal(t, 200, r.StatusCode)

		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		require.Equal(t, false, r.Uncompressed)

		require.Contains(t, string(b), "sample")
		_ = r.Body.Close()
	}
}
