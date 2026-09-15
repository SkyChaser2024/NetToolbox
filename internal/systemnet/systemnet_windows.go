package systemnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unsafe"

	"campusnet-toolbox/internal/winpaths"
	"golang.org/x/sys/windows"
)

const (
	PriorityAutomatic = "automatic"
	PriorityEthernet  = "ethernet"
	PriorityWiFi      = "wifi"
)

type NetworkInterface struct {
	Index         int      `json:"index"`
	AdapterID     string   `json:"-"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Kind          string   `json:"kind"`
	Physical      bool     `json:"physical"`
	Up            bool     `json:"up"`
	MAC           string   `json:"mac"`
	IPv4          []string `json:"ipv4"`
	IPv6          []string `json:"ipv6"`
	LinkSpeedMbps uint64   `json:"linkSpeedMbps"`
	IPv4Metric    uint32   `json:"ipv4Metric"`
	IPv6Metric    uint32   `json:"ipv6Metric"`
}

func List() ([]NetworkInterface, error) {
	var size uint32 = 15 * 1024
	flags := uint32(windows.GAA_FLAG_INCLUDE_ALL_INTERFACES | windows.GAA_FLAG_SKIP_ANYCAST | windows.GAA_FLAG_SKIP_MULTICAST | windows.GAA_FLAG_SKIP_DNS_SERVER)
	for attempt := 0; attempt < 3; attempt++ {
		buffer := make([]byte, size)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, first, &size)
		if errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("读取 Windows 网卡信息失败: %w", err)
		}
		result := make([]NetworkInterface, 0)
		for current := first; current != nil; current = current.Next {
			name := windows.UTF16PtrToString(current.FriendlyName)
			description := windows.UTF16PtrToString(current.Description)
			virtual := isVirtual(name + " " + description)
			kind := interfaceKind(current.IfType, virtual)
			physical := !virtual && (kind == PriorityEthernet || kind == PriorityWiFi)
			macLength := int(current.PhysicalAddressLength)
			if macLength > len(current.PhysicalAddress) {
				macLength = len(current.PhysicalAddress)
			}
			ipv4, ipv6 := unicastAddresses(current.FirstUnicastAddress)
			linkSpeed := current.ReceiveLinkSpeed
			if current.TransmitLinkSpeed < linkSpeed || linkSpeed == 0 {
				linkSpeed = current.TransmitLinkSpeed
			}
			result = append(result, NetworkInterface{
				Index:         int(current.IfIndex),
				AdapterID:     windows.BytePtrToString(current.AdapterName),
				Name:          name,
				Description:   description,
				Kind:          kind,
				Physical:      physical,
				Up:            current.OperStatus == windows.IfOperStatusUp,
				MAC:           net.HardwareAddr(current.PhysicalAddress[:macLength]).String(),
				IPv4:          ipv4,
				IPv6:          ipv6,
				LinkSpeedMbps: linkSpeed / 1_000_000,
				IPv4Metric:    current.Ipv4Metric,
				IPv6Metric:    current.Ipv6Metric,
			})
		}
		sort.SliceStable(result, func(i, j int) bool {
			if result[i].Kind != result[j].Kind {
				return result[i].Kind < result[j].Kind
			}
			return result[i].Name < result[j].Name
		})
		return result, nil
	}
	return nil, errors.New("读取 Windows 网卡信息时缓冲区持续变化，请稍后重试")
}

func unicastAddresses(first *windows.IpAdapterUnicastAddress) (ipv4, ipv6 []string) {
	ipv4 = make([]string, 0)
	ipv6 = make([]string, 0)
	for current := first; current != nil; current = current.Next {
		ip := current.Address.IP()
		if ip == nil || ip.IsUnspecified() {
			continue
		}
		if ip.To4() != nil {
			ipv4 = append(ipv4, ip.String())
		} else {
			ipv6 = append(ipv6, ip.String())
		}
	}
	return ipv4, ipv6
}

func ApplyPriority(mode string) error {
	if mode != PriorityAutomatic && mode != PriorityEthernet && mode != PriorityWiFi {
		return errors.New("未知的网卡优先级模式")
	}
	interfaces, err := List()
	if err != nil {
		return err
	}
	if len(interfaces) == 0 {
		return errors.New("未发现可调整的以太网或 Wi-Fi 接口")
	}

	lines := []string{"$ErrorActionPreference = 'Stop'"}
	for _, item := range interfaces {
		if !item.Physical || (item.Kind != PriorityEthernet && item.Kind != PriorityWiFi) {
			continue
		}
		if mode == PriorityAutomatic {
			lines = append(lines, metricCommand(item.Index, "IPv4", true, 0), metricCommand(item.Index, "IPv6", true, 0))
			continue
		}
		metric := 50
		if item.Kind == mode {
			metric = 10
		}
		lines = append(lines, metricCommand(item.Index, "IPv4", false, metric), metricCommand(item.Index, "IPv6", false, metric))
	}
	if len(lines) == 1 {
		return errors.New("未发现可调整的物理以太网或 Wi-Fi 接口")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tool, err := powershellPath()
	if err != nil {
		return fmt.Errorf("无法定位 Windows PowerShell: %w", err)
	}
	command := exec.CommandContext(ctx, tool, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", strings.Join(lines, "\n"))
	command.SysProcAttr = &windows.SysProcAttr{HideWindow: true}
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("调整网卡优先级失败: %s", message)
	}
	return nil
}

func interfaceKind(value uint32, virtual bool) string {
	if virtual {
		return "virtual"
	}
	switch value {
	case windows.IF_TYPE_ETHERNET_CSMACD:
		return PriorityEthernet
	case windows.IF_TYPE_IEEE80211:
		return PriorityWiFi
	case 24:
		return "loopback"
	case 131:
		return "tunnel"
	case 23:
		return "ppp"
	default:
		return "other"
	}
}

func metricCommand(index int, family string, automatic bool, metric int) string {
	if automatic {
		return fmt.Sprintf("Set-NetIPInterface -InterfaceIndex %d -AddressFamily %s -AutomaticMetric Enabled", index, family)
	}
	return fmt.Sprintf("Set-NetIPInterface -InterfaceIndex %d -AddressFamily %s -AutomaticMetric Disabled -InterfaceMetric %d", index, family, metric)
}

func powershellPath() (string, error) {
	return winpaths.SystemExecutable("WindowsPowerShell", "v1.0", "powershell.exe")
}

func isVirtual(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"virtual", "虚拟", "hyper-v", "vmware", "virtualbox", "vethernet",
		"wireguard", "wintun", "tap", "vpn", "tailscale", "zerotier",
		"docker", "wsl", "loopback", "npcap", "teredo", "isatap", "6to4",
		"pseudo", "bluetooth", "miniport", "tunnel", "kernel debugger", "内核调试器",
		"qos packet scheduler", "lightweight filter", "wfp ", "filter-",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
