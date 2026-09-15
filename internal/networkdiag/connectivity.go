package networkdiag

import (
	"context"
	"net"
	"net/http"
	"time"

	"campusnet-toolbox/internal/appmeta"
)

type connectivityProbe struct {
	URL            string
	ExpectedStatus int
	LocationHost   string
}

var connectivityProbes = []connectivityProbe{
	{URL: "https://www.baidu.com/favicon.ico", ExpectedStatus: http.StatusOK},
	{URL: "https://www.qq.com/favicon.ico", ExpectedStatus: http.StatusMovedPermanently, LocationHost: "mat1.gtimg.com"},
	{URL: "https://cp.cloudflare.com/generate_204", ExpectedStatus: http.StatusNoContent},
}

// InterfaceOnline verifies Internet reachability using a source address from
// one specific adapter. A successful HTTPS response cannot be supplied by a
// typical unauthenticated captive portal and therefore avoids mistaking Wi-Fi
// connectivity for an authenticated Ethernet connection.
func InterfaceOnline(ctx context.Context, addresses []string) bool {
	localIPs := usableLocalIPs(addresses)
	if len(localIPs) == 0 {
		return false
	}
	for _, localIP := range localIPs {
		for _, endpoint := range connectivityProbes {
			if probeFromAddress(ctx, localIP, endpoint) {
				return true
			}
			if ctx.Err() != nil {
				return false
			}
		}
	}
	return false
}

func usableLocalIPs(addresses []string) []net.IP {
	result := make([]net.IP, 0, len(addresses))
	for _, value := range addresses {
		ip := net.ParseIP(value)
		if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		result = append(result, ip)
	}
	return result
}

func probeFromAddress(ctx context.Context, localIP net.IP, endpoint connectivityProbe) bool {
	network := "tcp6"
	if localIP.To4() != nil {
		network = "tcp4"
	}
	dialer := &net.Dialer{
		Timeout:   1400 * time.Millisecond,
		LocalAddr: &net.TCPAddr{IP: localIP},
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(dialContext context.Context, _, address string) (net.Conn, error) {
			return dialer.DialContext(dialContext, network, address)
		},
		TLSHandshakeTimeout: 1400 * time.Millisecond,
		DisableKeepAlives:   true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   1500 * time.Millisecond,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint.URL, nil)
	if err != nil {
		return false
	}
	request.Header.Set("User-Agent", appmeta.UserAgent)
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	response.Body.Close()
	return connectivityResponseMatches(endpoint, response)
}

func connectivityResponseMatches(endpoint connectivityProbe, response *http.Response) bool {
	if response.StatusCode != endpoint.ExpectedStatus {
		return false
	}
	if endpoint.LocationHost == "" {
		return true
	}
	location, err := response.Location()
	return err == nil && location.Hostname() == endpoint.LocationHost
}
