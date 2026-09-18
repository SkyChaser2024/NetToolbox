package networkdiag

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchFirstPublicInfoPrefersSlowerDetailedProvider(t *testing.T) {
	detailed := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		time.Sleep(700 * time.Millisecond)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"ip":"1.1.1.1","isp":"Example ISP","asn":64500,"asn_organization":"Example Network","country":"CN","region":"Shanghai","city":"Shanghai"}`))
	}))
	defer detailed.Close()

	plain := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("1.1.1.1"))
	}))
	defer plain.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	providers := []publicInfoProvider{
		{url: detailed.URL, format: publicInfoIPSB},
		{url: plain.URL, format: publicInfoPlain},
	}
	info, err := fetchFirstPublicInfo(context.Background(), client, "tcp4", providers)
	if err != nil {
		t.Fatalf("fetchFirstPublicInfo returned an error: %v", err)
	}
	if info.Source != detailed.URL {
		t.Fatalf("expected detailed provider %q, got %q", detailed.URL, info.Source)
	}
	if info.ISP != "Example ISP" || info.ASN != 64500 || info.City != "Shanghai" {
		t.Fatalf("expected detailed metadata, got %+v", info)
	}
}

func TestFetchFirstPublicInfoFallsBackToPlainAddress(t *testing.T) {
	detailed := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "unavailable", http.StatusServiceUnavailable)
	}))
	defer detailed.Close()

	plain := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("1.1.1.1"))
	}))
	defer plain.Close()

	providers := []publicInfoProvider{
		{url: detailed.URL, format: publicInfoIPSB},
		{url: plain.URL, format: publicInfoPlain},
	}
	info, err := fetchFirstPublicInfo(context.Background(), &http.Client{Timeout: time.Second}, "tcp4", providers)
	if err != nil {
		t.Fatalf("fetchFirstPublicInfo returned an error: %v", err)
	}
	if !info.Available || info.Address != "1.1.1.1" || info.Source != plain.URL {
		t.Fatalf("expected plain address fallback, got %+v", info)
	}
}
