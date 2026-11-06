package backup

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/krkn-chaos/krknctl/pkg/backupstore"
)

const (
	DefaultDownloadTimeout = 5 * time.Minute
	DefaultMaxDownloadSize = 100 << 20 // 100 MB
	defaultMaxRedirects    = 10
)

// defaultTransport is nil in production; tests override it with a deterministic
// transport.
var defaultTransport http.RoundTripper

// lookupIP is a variable so tests can exercise hostname validation without
// depending on the machine's DNS configuration.
var lookupIP = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// DownloadArchive fetches a backup archive from an HTTPS URL into a temporary
// file. It enforces HTTPS-only, a maximum download size, a request timeout,
// and redirect safety. Remote hostnames are resolved and restricted to public
// addresses before every request and again at connection time to prevent DNS
// rebinding from reaching internal services. The caller must call cleanup
// when the temp file is no longer needed.
func DownloadArchive(ctx context.Context, rawURL string, maxSize int64, timeout time.Duration) (_ string, _ func(), retErr error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", nil, fmt.Errorf("invalid URL")
	}
	if maxSize <= 0 {
		maxSize = DefaultMaxDownloadSize
	}
	if timeout <= 0 {
		timeout = DefaultDownloadTimeout
	}

	downloadCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := validateRemoteURL(downloadCtx, parsed); err != nil {
		return "", nil, err
	}

	transport := defaultTransport
	if transport == nil {
		baseTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return "", nil, fmt.Errorf("unsupported default HTTP transport")
		}
		secureTransport := baseTransport.Clone()
		// Do not let an HTTP proxy resolve the destination on our behalf. The
		// dialer below must connect only to the validated IP addresses.
		secureTransport.Proxy = nil
		secureTransport.DialContext = dialValidated
		transport = secureTransport
	}

	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= defaultMaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			if err := validateRemoteURL(req.Context(), req.URL); err != nil {
				return fmt.Errorf("unsafe redirect: %w", err)
			}
			return nil
		},
	}

	dlReq, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to construct download request: %w", err)
	}

	resp, err := httpClient.Do(dlReq)
	if err != nil {
		return "", nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("remote server returned HTTP %d", resp.StatusCode)
	}

	return writeArchiveToTemp(resp.Body, maxSize, "krkn-restore-url-*.tar.gz")
}

func validateRemoteURL(ctx context.Context, parsed *url.URL) error {
	if parsed == nil || parsed.Host == "" {
		return fmt.Errorf("invalid URL")
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed")
	}
	if parsed.User != nil {
		return fmt.Errorf("URLs with embedded credentials are not allowed")
	}
	if _, err := resolveAndValidateHost(ctx, parsed.Hostname()); err != nil {
		return err
	}
	return nil
}

func resolveAndValidateHost(ctx context.Context, host string) ([]net.IP, error) {
	if host == "" {
		return nil, fmt.Errorf("URL hostname is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isDisallowedIP(ip) {
			return nil, fmt.Errorf("URL must not target a private or local address")
		}
		return []net.IP{ip}, nil
	}

	addresses, err := lookupIP(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve URL hostname: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("URL hostname has no addresses")
	}
	ips := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		if isDisallowedIP(address.IP) {
			return nil, fmt.Errorf("URL must not resolve to a private or local address")
		}
		ips = append(ips, address.IP)
	}
	return ips, nil
}

func isDisallowedIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified()
}

func dialValidated(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}
	ips, err := resolveAndValidateHost(ctx, host)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{}
	var lastErr error
	for _, ip := range ips {
		if network == "tcp4" && ip.To4() == nil {
			continue
		}
		if network == "tcp6" && ip.To4() != nil {
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no address matches network %q", network)
}

// DownloadArchiveFromStore reads an archive through a store's optional read
// capability and places it in a temporary file for normal restore processing.
// The caller must call cleanup after restore completes.
func DownloadArchiveFromStore(ctx context.Context, store backupstore.Reader, key string, maxSize int64, timeout time.Duration) (string, func(), error) {
	if store == nil {
		return "", nil, fmt.Errorf("archive store does not support downloads")
	}
	if timeout <= 0 {
		timeout = DefaultDownloadTimeout
	}
	readCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reader, size, err := store.Get(readCtx, key)
	if err != nil {
		return "", nil, err
	}
	if reader == nil {
		return "", nil, fmt.Errorf("archive store returned an empty reader")
	}
	defer reader.Close()
	if maxSize <= 0 {
		maxSize = DefaultMaxDownloadSize
	}
	if size > maxSize {
		return "", nil, fmt.Errorf("remote archive exceeds maximum size (%d bytes)", maxSize)
	}
	return writeArchiveToTemp(reader, maxSize, "krkn-restore-s3-*.tar.gz")
}

func writeArchiveToTemp(source io.Reader, maxSize int64, pattern string) (_ string, _ func(), retErr error) {
	tempFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempName := tempFile.Name()
	removeTemp := func() { os.Remove(tempName) }

	defer func() {
		if retErr != nil {
			tempFile.Close()
			removeTemp()
		}
	}()

	limited := io.LimitReader(source, maxSize+1)
	written, err := io.Copy(tempFile, limited)
	if err != nil {
		return "", nil, fmt.Errorf("error reading response body: %w", err)
	}
	if written > maxSize {
		return "", nil, fmt.Errorf("remote archive exceeds maximum size (%d bytes)", maxSize)
	}
	if err := tempFile.Close(); err != nil {
		return "", nil, fmt.Errorf("failed to finalize downloaded file: %w", err)
	}

	return tempName, removeTemp, nil
}
