package main

import (
	"strings"

	"campusnet-toolbox/internal/adapters"
	"campusnet-toolbox/internal/systemnet"
)

func selectEthernetAdapter(items []adapters.Adapter, preferredDevice string) *adapters.Adapter {
	for index := range items {
		item := &items[index]
		if item.Kind == "ethernet" && item.Recommended && item.DeviceName == preferredDevice {
			return item
		}
	}
	for index := range items {
		item := &items[index]
		if item.Kind == "ethernet" && item.Recommended && item.Up {
			return item
		}
	}
	for index := range items {
		item := &items[index]
		if item.Kind == "ethernet" && item.Recommended {
			return item
		}
	}
	return nil
}

func adapterAddresses(item *adapters.Adapter) []string {
	if item == nil {
		return nil
	}
	result := make([]string, 0, len(item.IPv4)+len(item.IPv6))
	result = append(result, item.IPv4...)
	result = append(result, item.IPv6...)
	return result
}

func selectSystemEthernet(items []systemnet.NetworkInterface, preferredDevice, preferredMAC string) *systemnet.NetworkInterface {
	normalizedMAC := normalizeMAC(preferredMAC)
	for index := range items {
		item := &items[index]
		deviceMatch := item.AdapterID != "" && strings.Contains(strings.ToLower(preferredDevice), strings.ToLower(item.AdapterID))
		macMatch := normalizedMAC != "" && normalizeMAC(item.MAC) == normalizedMAC
		if item.Physical && item.Kind == systemnet.PriorityEthernet && (deviceMatch || macMatch) {
			return item
		}
	}
	for index := range items {
		item := &items[index]
		if item.Physical && item.Kind == systemnet.PriorityEthernet && item.Up {
			return item
		}
	}
	for index := range items {
		item := &items[index]
		if item.Physical && item.Kind == systemnet.PriorityEthernet {
			return item
		}
	}
	return nil
}

func interfaceAddresses(item *systemnet.NetworkInterface) []string {
	if item == nil {
		return nil
	}
	result := make([]string, 0, len(item.IPv4)+len(item.IPv6))
	result = append(result, item.IPv4...)
	result = append(result, item.IPv6...)
	return result
}

func normalizeMAC(value string) string {
	value = strings.ToLower(value)
	return strings.NewReplacer(":", "", "-", "", ".", "").Replace(value)
}
