package adapters

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"campusnet-toolbox/internal/systemnet"
	"github.com/google/gopacket/pcap"
)

type Adapter struct {
	DeviceName     string   `json:"deviceName"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	MAC            string   `json:"mac"`
	IPv4           []string `json:"ipv4"`
	IPv6           []string `json:"ipv6"`
	Up             bool     `json:"up"`
	Recommended    bool     `json:"recommended"`
	InterfaceIndex int      `json:"interfaceIndex"`
	Kind           string   `json:"kind"`
}

type Result struct {
	Adapters       []Adapter `json:"adapters"`
	NpcapAvailable bool      `json:"npcapAvailable"`
	Error          string    `json:"error,omitempty"`
}

type systemInterface struct {
	name      string
	mac       string
	up        bool
	addrs     map[string]struct{}
	adapterID string
	index     int
	kind      string
}

func List() Result {
	return listWithSystemInterfaces(listSystemInterfaces())
}

// ListWithInterfaces reuses a Windows adapter snapshot already collected by
// the caller. Startup needs the same metadata for both the UI and Npcap
// matching, so this avoids a second GetAdaptersAddresses allocation and scan.
func ListWithInterfaces(items []systemnet.NetworkInterface) Result {
	return listWithSystemInterfaces(systemInterfaces(items))
}

func listWithSystemInterfaces(system []systemInterface) Result {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return Result{Adapters: make([]Adapter, 0), Error: fmt.Sprintf("未检测到可用的 Npcap: %v", err)}
	}
	result := Result{NpcapAvailable: true, Adapters: make([]Adapter, 0, len(devices))}
	for _, device := range devices {
		adapter := Adapter{DeviceName: device.Name, Description: device.Description, Up: true}
		deviceIPs := make(map[string]struct{})
		for _, address := range device.Addresses {
			if address.IP == nil {
				continue
			}
			ip := address.IP.String()
			deviceIPs[ip] = struct{}{}
			if address.IP.To4() != nil {
				adapter.IPv4 = append(adapter.IPv4, ip)
			} else {
				adapter.IPv6 = append(adapter.IPv6, ip)
			}
		}
		for _, candidate := range system {
			deviceMatch := candidate.adapterID != "" && strings.Contains(strings.ToLower(device.Name), strings.ToLower(candidate.adapterID))
			if deviceMatch || intersects(deviceIPs, candidate.addrs) {
				adapter.Name = candidate.name
				adapter.MAC = candidate.mac
				adapter.Up = candidate.up
				adapter.InterfaceIndex = candidate.index
				adapter.Kind = candidate.kind
				break
			}
		}
		if adapter.Name == "" {
			adapter.Name = adapter.Description
		}
		if adapter.Name == "" {
			adapter.Name = shortDeviceName(device.Name)
		}
		lower := strings.ToLower(adapter.Name + " " + adapter.Description)
		adapter.Recommended = adapter.MAC != "" && (adapter.Kind == systemnet.PriorityEthernet || adapter.Kind == systemnet.PriorityWiFi) &&
			!strings.Contains(lower, "loopback") && !strings.Contains(lower, "virtual") &&
			!strings.Contains(lower, "bluetooth")
		result.Adapters = append(result.Adapters, adapter)
	}
	sort.SliceStable(result.Adapters, func(i, j int) bool {
		if result.Adapters[i].Recommended != result.Adapters[j].Recommended {
			return result.Adapters[i].Recommended
		}
		if result.Adapters[i].Up != result.Adapters[j].Up {
			return result.Adapters[i].Up
		}
		return result.Adapters[i].Name < result.Adapters[j].Name
	})
	return result
}

func listSystemInterfaces() []systemInterface {
	metadata, metadataErr := systemnet.List()
	if metadataErr == nil {
		return systemInterfaces(metadata)
	}

	interfaces, _ := net.Interfaces()
	result := make([]systemInterface, 0, len(interfaces))
	for _, item := range interfaces {
		entry := systemInterface{name: item.Name, mac: item.HardwareAddr.String(), up: item.Flags&net.FlagUp != 0, addrs: map[string]struct{}{}, index: item.Index}
		addresses, _ := item.Addrs()
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil {
				entry.addrs[ip.String()] = struct{}{}
			}
		}
		result = append(result, entry)
	}
	return result
}

func systemInterfaces(items []systemnet.NetworkInterface) []systemInterface {
	result := make([]systemInterface, 0, len(items))
	for _, item := range items {
		addresses := make(map[string]struct{}, len(item.IPv4)+len(item.IPv6))
		for _, address := range item.IPv4 {
			addresses[address] = struct{}{}
		}
		for _, address := range item.IPv6 {
			addresses[address] = struct{}{}
		}
		result = append(result, systemInterface{
			name: item.Name, mac: item.MAC, up: item.Up, addrs: addresses,
			adapterID: item.AdapterID, index: item.Index, kind: item.Kind,
		})
	}
	return result
}

func intersects(left, right map[string]struct{}) bool {
	for item := range left {
		if _, ok := right[item]; ok {
			return true
		}
	}
	return false
}

func shortDeviceName(value string) string {
	if index := strings.LastIndex(value, "_"); index >= 0 && index+1 < len(value) {
		return value[index+1:]
	}
	return value
}
