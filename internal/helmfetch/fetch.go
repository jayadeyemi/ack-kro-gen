package helmfetch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
)

// EnsureChart downloads the chart into cache if not present and returns the local path to a chart archive.
// chartRef must include version, e.g. oci://.../ack-ec2-controller-chart:1.2.27
func EnsureChart(ctx context.Context, chartRef, cacheDir string, offline bool) (string, error) {
	if !strings.HasPrefix(chartRef, "oci://") {
		return "", errors.New("only OCI chart refs are supported")
	}
	if _, err := url.Parse(chartRef); err != nil { return "", fmt.Errorf("invalid chart ref: %w", err) }
	if err := os.MkdirAll(cacheDir, 0o755); err != nil { return "", err }

	settings := cli.New()
	settings.RepositoryCache = filepath.Join(cacheDir, "repo-cache")
	settings.RepositoryConfig = filepath.Join(cacheDir, "repositories.yaml")
	settings.RegistryConfig = filepath.Join(cacheDir, "registry.json")

	rc, err := registry.NewClient(registry.ClientOptDebug(false))
	if err != nil { return "", fmt.Errorf("registry client: %w", err) }

	providers := getter.All(settings)
	cd := downloader.ChartDownloader{
		Out:              os.Stdout,
		Getters:          providers,
		Options:          []getter.Option{},
		RegistryClient:   rc,
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
	}

	// Helm downloader will write into current cwd by default. Force cacheDir.
	pwd := cacheDir
	name, version := splitOCI(chartRef)
	if name == "" || version == "" { return "", errors.New("oci ref must include :version") }
	archive := filepath.Join(pwd, fmt.Sprintf("%s-%s.tgz", sanitize(filepath.Base(name)), version))

	if fi, err := os.Stat(archive); err == nil && fi.Size() > 0 {
		return archive, nil
	}
	if offline {
		return "", fmt.Errorf("offline mode and chart not cached: %s", archive)
	}
	if _, _, err := cd.DownloadTo(name, version, pwd); err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	if _, err := os.Stat(archive); err != nil {
		return "", fmt.Errorf("downloaded chart archive missing: %s", archive)
	}
	return archive, nil
}

func splitOCI(ref string) (string, string) {
	// oci://host/path:tag
	i := strings.LastIndex(ref, ":")
	if i < 0 { return ref, "" }
	return ref[:i], ref[i+1:]
}

func sanitize(s string) string { return strings.Map(func(r rune) rune {
	if r == '/' || r == ':' { return '_' }
	return r
}, s) }