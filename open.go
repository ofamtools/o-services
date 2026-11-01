package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"runtime"
	"strconv"
)

func browserURL(e Endpoint, scheme string) (string, error) {
	if scheme != "http" && scheme != "https" {
		return "", errors.New("choose scheme http or https; a port does not advertise a web protocol")
	}
	if e.Transport != "tcp" {
		return "", errors.New("only TCP endpoints can be opened in a browser; UDP is a bound socket")
	}
	if e.Port == 0 {
		return "", errors.New("endpoint has no local port")
	}
	address, err := netip.ParseAddr(e.Address)
	if err != nil {
		return "", fmt.Errorf("invalid local address: %w", err)
	}
	if (e.Family != "ipv4" && e.Family != "ipv6") || (e.Family == "ipv4") != address.Is4() {
		return "", errors.New("endpoint address family mismatch")
	}
	if address.IsUnspecified() {
		if address.Is4() {
			address = netip.MustParseAddr("127.0.0.1")
		} else {
			address = netip.IPv6Loopback()
		}
	}
	return (&url.URL{Scheme: scheme, Host: net.JoinHostPort(address.String(), strconv.Itoa(int(e.Port))), Path: "/"}).String(), nil
}

func validatedURL(ctx context.Context, service Service, endpoint Endpoint, scheme string, scan scanner) (string, error) {
	if service.Stale {
		return "", errors.New("selected service is stale; refresh successfully before opening")
	}
	if !service.Verified() {
		return "", errors.New("selected service has no verified process birth")
	}
	target, err := browserURL(endpoint, scheme)
	if err != nil {
		return "", err
	}
	before, err := scan.Process(service.PID)
	if err != nil || before.Birth != service.Birth {
		return "", errors.New("selected process exited or changed identity")
	}
	current := scan.Discover(ctx)
	if !current.Complete {
		return "", fmt.Errorf("cannot validate endpoint against an incomplete discovery: %v", current.Problems)
	}
	live := false
	for _, p := range current.Services {
		if p.ID == service.ID && p.Birth == service.Birth {
			live = true
			break
		}
	}
	if !live {
		return "", errors.New("selected service is no longer present")
	}
	found := false
	for _, e := range current.Endpoints {
		if e.Service == service.ID && e.sameSocket(endpoint) {
			found = true
			break
		}
	}
	if !found {
		return "", errors.New("selected endpoint is no longer bound by this service")
	}
	after, err := scan.Process(service.PID)
	if err != nil || after.Birth != service.Birth {
		return "", errors.New("selected process changed during endpoint validation")
	}
	return target, nil
}

func openBrowser(ctx context.Context, target string) error {
	switch runtime.GOOS {
	case "darwin":
		return command(ctx, "open", "--", target).error("browser opener")
	case "linux":
		return command(ctx, "xdg-open", target).error("browser opener")
	default:
		return fmt.Errorf("browser opening unsupported on %s", runtime.GOOS)
	}
}
