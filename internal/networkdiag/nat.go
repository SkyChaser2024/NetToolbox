package networkdiag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/pion/stun/v3"
)

type NATProbe struct {
	Server   string `json:"server"`
	ServerIP string `json:"serverIp,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Latency  int64  `json:"latencyMs,omitempty"`
	Error    string `json:"error,omitempty"`
}

type NATResult struct {
	Status            string     `json:"status"`
	Type              string     `json:"type"`
	Summary           string     `json:"summary"`
	LocalIP           string     `json:"localIp"`
	PublicIP          string     `json:"publicIp"`
	PublicPort        int        `json:"publicPort"`
	MappingBehavior   string     `json:"mappingBehavior"`
	FilteringBehavior string     `json:"filteringBehavior"`
	LatencyMs         int64      `json:"latencyMs"`
	ServerCount       int        `json:"serverCount"`
	RFC5780           bool       `json:"rfc5780"`
	Probes            []NATProbe `json:"probes"`
	Error             string     `json:"error,omitempty"`
}

var errNoOtherAddress = errors.New("STUN 服务器不支持 RFC 5780 OTHER-ADDRESS")

const maxNATServers = 16

func CheckNAT(ctx context.Context, servers []string) NATResult {
	if len(servers) == 0 {
		return NATResult{Status: "error", Type: "未知", Error: "未配置 STUN 服务器"}
	}
	if len(servers) > maxNATServers {
		servers = servers[:maxNATServers]
	}
	type discoveryOutcome struct {
		result NATResult
		err    error
	}
	discoveryDone := make(chan discoveryOutcome, 1)
	go func() {
		var lastErr error
		for _, server := range servers {
			result, err := behaviorDiscovery(ctx, server)
			if err == nil {
				discoveryDone <- discoveryOutcome{result: result}
				return
			}
			lastErr = err
			if ctx.Err() != nil {
				break
			}
		}
		discoveryDone <- discoveryOutcome{err: lastErr}
	}()

	fallback := basicMappingDiscovery(ctx, servers)
	var discovery discoveryOutcome
	select {
	case discovery = <-discoveryDone:
	case <-ctx.Done():
		discovery.err = ctx.Err()
	}
	if discovery.err == nil {
		if len(fallback.Probes) > 0 {
			serverCount := fallback.ServerCount
			if len(discovery.result.Probes) > 0 {
				completeProbe := discovery.result.Probes[0]
				matched := false
				for index := range fallback.Probes {
					if fallback.Probes[index].Server == completeProbe.Server {
						matched = true
						if fallback.Probes[index].Error != "" {
							fallback.Probes[index] = completeProbe
							serverCount++
						}
						break
					}
				}
				if !matched {
					fallback.Probes = append(fallback.Probes, completeProbe)
					serverCount++
				}
			}
			discovery.result.Probes = fallback.Probes
			discovery.result.ServerCount = serverCount
		}
		return discovery.result
	}
	if fallback.Status == "error" {
		fallback.Error = fmt.Sprintf("RFC 5780 检测失败（%v）；备用 STUN 检测也未成功", discovery.err)
	} else {
		fallback.Summary += "；当前公共 STUN 节点不支持完整过滤行为测试，结果为保守判断"
	}
	return fallback
}

func behaviorDiscovery(ctx context.Context, server string) (NATResult, error) {
	remote, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return NATResult{}, err
	}
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return NATResult{}, err
	}
	defer conn.Close()
	localIP := localIPv4For(remote)
	start := time.Now()
	first, err := stunRoundTrip(ctx, conn, remote, nil)
	if err != nil {
		return NATResult{}, err
	}
	mapped, other, err := stunAddresses(first)
	if err != nil {
		return NATResult{}, err
	}
	if other == nil || other.IP == nil || other.IP.IsPrivate() || other.IP.IsUnspecified() {
		return NATResult{}, errNoOtherAddress
	}

	result := NATResult{
		Status: "ok", LocalIP: localIP.String(), PublicIP: mapped.IP.String(), PublicPort: mapped.Port,
		LatencyMs: time.Since(start).Milliseconds(), ServerCount: 1, RFC5780: true,
		Probes: []NATProbe{{Server: server, ServerIP: remote.IP.String(), Endpoint: mapped.String(), Latency: time.Since(start).Milliseconds()}},
	}
	localPort := conn.LocalAddr().(*net.UDPAddr).Port
	if localIP.Equal(mapped.IP) && localPort == mapped.Port {
		result.MappingBehavior = "无地址转换"
	} else {
		secondTarget := &net.UDPAddr{IP: other.IP, Port: remote.Port}
		second, secondErr := stunRoundTrip(ctx, conn, secondTarget, nil)
		if secondErr != nil {
			return NATResult{}, secondErr
		}
		mappedSecond, _, secondErr := stunAddresses(second)
		if secondErr != nil {
			return NATResult{}, secondErr
		}
		if sameEndpoint(mapped, mappedSecond) {
			result.MappingBehavior = "端点无关映射"
		} else {
			third, thirdErr := stunRoundTrip(ctx, conn, other, nil)
			if thirdErr != nil {
				return NATResult{}, thirdErr
			}
			mappedThird, _, thirdErr := stunAddresses(third)
			if thirdErr != nil {
				return NATResult{}, thirdErr
			}
			if sameEndpoint(mappedSecond, mappedThird) {
				result.MappingBehavior = "地址相关映射"
			} else {
				result.MappingBehavior = "地址与端口相关映射"
			}
		}
	}

	filtering, filterErr := discoverFiltering(ctx, remote)
	if filterErr != nil {
		return NATResult{}, filterErr
	}
	result.FilteringBehavior = filtering
	classifyNAT(&result, localIP, localPort, mapped)
	return result, nil
}

func discoverFiltering(ctx context.Context, remote *net.UDPAddr) (string, error) {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	first, err := stunRoundTrip(ctx, conn, remote, nil)
	if err != nil {
		return "", err
	}
	_, other, err := stunAddresses(first)
	if err != nil || other == nil {
		return "", errNoOtherAddress
	}
	changeBoth := []byte{0, 0, 0, 0x06}
	if _, err := stunRoundTrip(ctx, conn, remote, changeBoth); err == nil {
		return "端点无关过滤", nil
	}
	changePort := []byte{0, 0, 0, 0x02}
	if _, err := stunRoundTrip(ctx, conn, remote, changePort); err == nil {
		return "地址相关过滤", nil
	}
	return "地址与端口相关过滤", nil
}

func basicMappingDiscovery(ctx context.Context, servers []string) NATResult {
	result := NATResult{Status: "error", Type: "未知", Summary: "UDP/STUN 无响应", MappingBehavior: "无法判断", FilteringBehavior: "无法判断"}
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer conn.Close()
	localIP := net.IPv4zero
	var mapped []*net.UDPAddr
	for _, server := range servers {
		probe := NATProbe{Server: server}
		remote, resolveErr := net.ResolveUDPAddr("udp4", server)
		if resolveErr != nil {
			probe.Error = resolveErr.Error()
			result.Probes = append(result.Probes, probe)
			continue
		}
		probe.ServerIP = remote.IP.String()
		if localIP.IsUnspecified() {
			localIP = localIPv4For(remote)
			result.LocalIP = localIP.String()
		}
		started := time.Now()
		message, probeErr := stunRoundTrip(ctx, conn, remote, nil)
		probe.Latency = time.Since(started).Milliseconds()
		if probeErr != nil {
			probe.Error = probeErr.Error()
			result.Probes = append(result.Probes, probe)
			continue
		}
		endpoint, _, parseErr := stunAddresses(message)
		if parseErr != nil {
			probe.Error = parseErr.Error()
		} else {
			if len(mapped) == 0 {
				result.LatencyMs = probe.Latency
			}
			mapped = append(mapped, endpoint)
			probe.Endpoint = endpoint.String()
		}
		result.Probes = append(result.Probes, probe)
	}
	if len(mapped) == 0 {
		result.Error = "所有 STUN 节点均未返回有效映射，可能是 UDP 被阻断"
		return result
	}
	result.Status = "ok"
	result.ServerCount = len(mapped)
	result.PublicIP, result.PublicPort = mapped[0].IP.String(), mapped[0].Port
	localPort := conn.LocalAddr().(*net.UDPAddr).Port
	if localIP.Equal(mapped[0].IP) && localPort == mapped[0].Port {
		result.Type, result.MappingBehavior = "开放网络（无 NAT）", "无地址转换"
		result.Summary = "本机拥有可直接观察到的公网 IPv4 地址"
		return result
	}
	stable := true
	for _, endpoint := range mapped[1:] {
		stable = stable && sameEndpoint(mapped[0], endpoint)
	}
	if stable {
		result.Type = "非对称 NAT（NAT1–3）"
		result.MappingBehavior = "端点无关映射"
		result.Summary = "不同 STUN 目标观察到相同映射，适合大多数 P2P 场景"
	} else {
		result.Type = "对称型 NAT（NAT4）"
		result.MappingBehavior = "地址与端口相关映射"
		result.Summary = "不同目标观察到不同公网端口，P2P 直连通常较困难"
	}
	return result
}

func classifyNAT(result *NATResult, localIP net.IP, localPort int, mapped *net.UDPAddr) {
	switch {
	case localIP.Equal(mapped.IP) && localPort == mapped.Port:
		result.Type = "开放网络（无 NAT）"
		result.Summary = "本机拥有可直接观察到的公网 IPv4 地址"
	case result.MappingBehavior == "地址与端口相关映射":
		result.Type = "对称型 NAT（NAT4）"
		result.Summary = "公网端口随目标变化，P2P 直连通常较困难"
	case result.FilteringBehavior == "端点无关过滤":
		result.Type = "全锥形 NAT（NAT1）"
		result.Summary = "映射与过滤限制较少，P2P 兼容性最佳"
	case result.FilteringBehavior == "地址相关过滤":
		result.Type = "受限锥形 NAT（NAT2）"
		result.Summary = "仅允许已联系过的远端地址回包"
	default:
		result.Type = "端口受限锥形 NAT（NAT3）"
		result.Summary = "仅允许已联系过的远端地址和端口回包"
	}
}

func stunRoundTrip(ctx context.Context, conn *net.UDPConn, remote *net.UDPAddr, changeRequest []byte) (*stun.Message, error) {
	request := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	if changeRequest != nil {
		request.Add(stun.AttrChangeRequest, changeRequest)
	}
	request.Encode()
	deadline := time.Now().Add(1400 * time.Millisecond)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if _, err := conn.WriteToUDP(request.Raw, remote); err != nil {
		return nil, err
	}
	buffer := make([]byte, 1500)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return nil, err
		}
		response := new(stun.Message)
		response.Raw = append([]byte(nil), buffer[:count]...)
		if err := response.Decode(); err != nil || response.TransactionID != request.TransactionID {
			continue
		}
		return response, nil
	}
}

func stunAddresses(message *stun.Message) (*net.UDPAddr, *net.UDPAddr, error) {
	xor := &stun.XORMappedAddress{}
	if err := xor.GetFrom(message); err != nil {
		mapped := &stun.MappedAddress{}
		if mappedErr := mapped.GetFrom(message); mappedErr != nil {
			return nil, nil, errors.New("STUN 响应缺少公网映射地址")
		}
		xor.IP, xor.Port = mapped.IP, mapped.Port
	}
	var otherAddress *net.UDPAddr
	other := &stun.OtherAddress{}
	if err := other.GetFrom(message); err == nil {
		otherAddress = &net.UDPAddr{IP: other.IP, Port: other.Port}
	}
	return &net.UDPAddr{IP: xor.IP, Port: xor.Port}, otherAddress, nil
}

func localIPv4For(remote *net.UDPAddr) net.IP {
	conn, err := net.DialUDP("udp4", nil, remote)
	if err == nil {
		defer conn.Close()
		return conn.LocalAddr().(*net.UDPAddr).IP
	}
	return net.IPv4zero
}

func sameEndpoint(left, right *net.UDPAddr) bool {
	return left != nil && right != nil && left.IP.Equal(right.IP) && left.Port == right.Port
}
