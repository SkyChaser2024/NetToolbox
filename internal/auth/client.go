package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket/pcap"
)

var paeGroupAddress = net.HardwareAddr{0x01, 0x80, 0xc2, 0x00, 0x00, 0x03}

const (
	MaxStartDelay      = 30 * time.Second
	MaxRetryDelay      = 60 * time.Second
	DefaultMaxAttempts = 3
	maxCredentialBytes = 4096
)

var ErrAttemptsExhausted = errors.New("认证未成功，已达到最多 3 次尝试")

type State string

const (
	StateIdle             State = "idle"
	StateStarting         State = "starting"
	StateWaitingIdentity  State = "waiting_identity"
	StateWaitingChallenge State = "waiting_challenge"
	StateAuthenticated    State = "authenticated"
	StateFailed           State = "failed"
	StateStopping         State = "stopping"
	StateError            State = "error"
)

type Config struct {
	DeviceName       string
	LocalMAC         string
	Username         string
	Password         string
	Identity         string
	IdentitySuffix   string
	StartDelay       time.Duration
	RetryDelay       time.Duration
	AuthenticationTO time.Duration
	MaxAttempts      int
	Debug            bool
}

type Event struct {
	State     State  `json:"state"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type EventSink func(Event)

func Validate(cfg Config) error {
	if strings.TrimSpace(cfg.DeviceName) == "" {
		return errors.New("请选择用于认证的网卡")
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return errors.New("请输入校园网账号")
	}
	if len(cfg.Username) > maxCredentialBytes {
		return errors.New("校园网账号过长")
	}
	if cfg.Password == "" {
		return errors.New("请输入校园网密码")
	}
	if len(cfg.Password) > maxCredentialBytes {
		return errors.New("校园网密码过长")
	}
	mac, err := net.ParseMAC(cfg.LocalMAC)
	if err != nil || len(mac) != 6 {
		return errors.New("网卡 MAC 地址无效")
	}
	if _, err := BuildIdentity(cfg.Identity, cfg.IdentitySuffix); err != nil {
		return fmt.Errorf("identity 扩展无效: %w", err)
	}
	if cfg.StartDelay < 0 || cfg.StartDelay > MaxStartDelay ||
		cfg.RetryDelay < 0 || cfg.RetryDelay > MaxRetryDelay {
		return errors.New("认证延迟或重试参数超出允许范围")
	}
	if cfg.MaxAttempts < 0 || cfg.MaxAttempts > DefaultMaxAttempts {
		return errors.New("认证尝试次数超出允许范围")
	}
	return nil
}

type Session struct {
	cfg      Config
	handle   *pcap.Handle
	localMAC net.HardwareAddr
	dstMAC   net.HardwareAddr
	sink     EventSink

	writeMu   sync.Mutex
	dstMu     sync.Mutex
	closeOnce sync.Once
}

func NewSession(cfg Config, sink EventSink) (*Session, error) {
	if cfg.Identity == "" {
		cfg.Identity = cfg.Username
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 2 * time.Second
	}
	if cfg.AuthenticationTO == 0 {
		cfg.AuthenticationTO = 8 * time.Second
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = DefaultMaxAttempts
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	mac, _ := net.ParseMAC(cfg.LocalMAC)
	handle, err := pcap.OpenLive(cfg.DeviceName, 1600, true, 350*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("无法打开 Npcap 网卡: %w", err)
	}
	if err := handle.SetBPFFilter("ether proto 0x888e"); err != nil {
		handle.Close()
		return nil, fmt.Errorf("无法设置 EAPOL 抓包过滤器: %w", err)
	}
	return &Session{
		cfg:      cfg,
		handle:   handle,
		localMAC: cloneMAC(mac),
		dstMAC:   cloneMAC(paeGroupAddress),
		sink:     sink,
	}, nil
}

func (s *Session) emit(state State, level, message string) {
	if s.sink == nil {
		return
	}
	s.sink(Event{State: state, Level: level, Message: message, Timestamp: time.Now().Format("15:04:05")})
}

func (s *Session) Run(ctx context.Context) error {
	if s.cfg.StartDelay > 0 {
		s.emit(StateStarting, "info", "正在等待设定的启动延迟…")
		timer := time.NewTimer(s.cfg.StartDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}

	for attempt := 1; attempt <= s.cfg.MaxAttempts; attempt++ {
		if attempt > 1 {
			timer := time.NewTimer(s.cfg.RetryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
		if err := s.sendStart(); err != nil {
			return fmt.Errorf("发送 EAPOL-Start 失败: %w", err)
		}
		if attempt == 1 {
			s.emit(StateWaitingIdentity, "info", "已发送 EAPOL-Start，等待交换机响应")
		} else {
			s.emit(StateWaitingIdentity, "info", fmt.Sprintf("正在进行第 %d/%d 次认证尝试", attempt, s.cfg.MaxAttempts))
		}
		deadline := time.Now().Add(s.cfg.AuthenticationTO)
		failed := false
		for time.Now().Before(deadline) && !failed {
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			data, _, err := s.handle.ReadPacketData()
			if err == nil {
				result, packetErr := s.handlePacket(data)
				if packetErr != nil {
					s.emit(StateError, "error", packetErr.Error())
				}
				if result.success {
					// EAPOL authentication is a one-shot exchange. Keeping the pcap
					// handle open after Success wastes resources and is not required
					// by the authenticator, so finish the session immediately.
					return nil
				}
				if result.failure {
					failed = true
				}
			} else if err != pcap.NextErrorTimeoutExpired {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("读取 EAPOL 报文失败: %w", err)
			}

		}
		if attempt < s.cfg.MaxAttempts {
			s.emit(StateWaitingIdentity, "warning", "本次认证未完成，将按设定延迟重试")
		}
	}
	s.emit(StateFailed, "error", ErrAttemptsExhausted.Error())
	return ErrAttemptsExhausted
}

type packetResult struct {
	success bool
	failure bool
}

func (s *Session) handlePacket(data []byte) (packetResult, error) {
	frame, ok, err := parseEAPOLFrame(data)
	if err != nil {
		return packetResult{}, err
	}
	if !ok {
		return packetResult{}, nil
	}
	if bytes.Equal(frame.sourceMAC, s.localMAC) {
		return packetResult{}, nil
	}
	if len(frame.sourceMAC) == 6 {
		s.dstMu.Lock()
		s.dstMAC = cloneMAC(frame.sourceMAC)
		s.dstMu.Unlock()
	}
	if s.cfg.Debug {
		s.emit(StateStarting, "debug", fmt.Sprintf("收到 EAP code=%d id=%d type=%d", frame.code, frame.identifier, frame.eapType))
	}

	switch frame.code {
	case 1: // Request
		switch frame.eapType {
		case 1:
			identity, err := BuildIdentity(s.cfg.Identity, s.cfg.IdentitySuffix)
			if err != nil {
				return packetResult{}, err
			}
			response, err := BuildIdentityResponse(frame.identifier, identity)
			if err != nil {
				return packetResult{}, err
			}
			defer clear(response)
			if err := s.writeEAP(s.destinationMAC(), response); err != nil {
				return packetResult{}, fmt.Errorf("发送 Identity 响应失败: %w", err)
			}
			s.emit(StateWaitingChallenge, "info", "已响应账号身份，等待 MD5 Challenge")
		case 4:
			if len(frame.typeData) < 1 {
				return packetResult{}, errors.New("收到的 MD5 Challenge 太短")
			}
			challengeLen := int(frame.typeData[0])
			if challengeLen == 0 || len(frame.typeData) < 1+challengeLen {
				return packetResult{}, errors.New("收到的 MD5 Challenge 格式不完整")
			}
			response, err := BuildMD5Response(frame.identifier, s.cfg.Password, frame.typeData[1:1+challengeLen], []byte(s.cfg.Username))
			if err != nil {
				return packetResult{}, err
			}
			defer clear(response)
			if err := s.writeEAP(s.destinationMAC(), response); err != nil {
				return packetResult{}, fmt.Errorf("发送 MD5 响应失败: %w", err)
			}
			s.emit(StateWaitingChallenge, "info", "已提交 EAP-MD5 响应，等待认证结果")
		default:
			s.emit(StateStarting, "warning", fmt.Sprintf("暂不支持交换机请求的 EAP 类型 %d", frame.eapType))
		}
	case 3: // Success
		s.emit(StateAuthenticated, "success", "802.1X 认证成功，网络已接入")
		return packetResult{success: true}, nil
	case 4: // Failure
		s.emit(StateWaitingIdentity, "warning", "802.1X 认证被拒绝")
		return packetResult{failure: true}, nil
	}
	return packetResult{}, nil
}

type parsedEAPOLFrame struct {
	sourceMAC  net.HardwareAddr
	code       byte
	identifier byte
	eapType    byte
	typeData   []byte
}

func parseEAPOLFrame(data []byte) (parsedEAPOLFrame, bool, error) {
	if len(data) < 18 {
		return parsedEAPOLFrame{}, false, nil
	}
	payloadOffset := 14
	etherType := uint16(data[12])<<8 | uint16(data[13])
	if etherType == 0x8100 || etherType == 0x88a8 {
		if len(data) < 22 {
			return parsedEAPOLFrame{}, false, nil
		}
		etherType = uint16(data[16])<<8 | uint16(data[17])
		payloadOffset = 18
	}
	if etherType != 0x888e {
		return parsedEAPOLFrame{}, false, nil
	}
	payload := data[payloadOffset:]
	if len(payload) < 4 || payload[1] != eapolEAP {
		return parsedEAPOLFrame{}, false, nil
	}
	eapolLength := int(payload[2])<<8 | int(payload[3])
	if eapolLength < 4 || len(payload) < 4+eapolLength {
		return parsedEAPOLFrame{}, false, errors.New("收到的 EAPOL 报文长度无效")
	}
	eap := payload[4 : 4+eapolLength]
	eapLength := int(eap[2])<<8 | int(eap[3])
	if eapLength < 4 || eapLength > len(eap) {
		return parsedEAPOLFrame{}, false, errors.New("收到的 EAP 报文长度无效")
	}
	result := parsedEAPOLFrame{
		sourceMAC:  cloneMAC(net.HardwareAddr(data[6:12])),
		code:       eap[0],
		identifier: eap[1],
	}
	if result.code == 1 || result.code == 2 {
		if eapLength < 5 {
			return parsedEAPOLFrame{}, false, errors.New("收到的 EAP 请求缺少类型字段")
		}
		result.eapType = eap[4]
		result.typeData = eap[5:eapLength]
	}
	return result, true, nil
}

func (s *Session) sendStart() error {
	return s.writeFrame(paeGroupAddress, buildEAPOL(eapolStart, nil))
}

func (s *Session) Logoff() error {
	return s.writeFrame(s.destinationMAC(), buildEAPOL(eapolLogoff, nil))
}

// SendLogoff opens the adapter only long enough to send a standards-compliant
// EAPOL-Logoff. This allows a successful authentication session to release its
// pcap handle while still supporting an explicit user logout later.
func SendLogoff(deviceName, localMAC string) error {
	if strings.TrimSpace(deviceName) == "" {
		return errors.New("没有可用于注销的网卡")
	}
	mac, err := net.ParseMAC(localMAC)
	if err != nil || len(mac) != 6 {
		return errors.New("网卡 MAC 地址无效")
	}
	handle, err := pcap.OpenLive(deviceName, 1600, false, 250*time.Millisecond)
	if err != nil {
		return fmt.Errorf("无法打开 Npcap 网卡: %w", err)
	}
	defer handle.Close()
	frame := make([]byte, 18)
	copy(frame[0:6], paeGroupAddress)
	copy(frame[6:12], mac)
	frame[12], frame[13] = 0x88, 0x8e
	copy(frame[14:], buildEAPOL(eapolLogoff, nil))
	if err := handle.WritePacketData(frame); err != nil {
		return fmt.Errorf("发送 EAPOL-Logoff 失败: %w", err)
	}
	return nil
}

func (s *Session) destinationMAC() net.HardwareAddr {
	s.dstMu.Lock()
	defer s.dstMu.Unlock()
	dst := s.dstMAC
	if len(dst) != 6 {
		dst = paeGroupAddress
	}
	return cloneMAC(dst)
}

func (s *Session) writeEAP(dst net.HardwareAddr, eap []byte) error {
	return s.writeFrame(dst, buildEAPOL(eapolEAP, eap))
}

func (s *Session) writeFrame(dst net.HardwareAddr, payload []byte) error {
	frame := make([]byte, 14+len(payload))
	copy(frame[0:6], dst)
	copy(frame[6:12], s.localMAC)
	frame[12], frame[13] = 0x88, 0x8e
	copy(frame[14:], payload)
	if s.cfg.Debug {
		// Raw Identity and MD5 response bytes can help an attacker perform an
		// offline password guess. Keep protocol diagnostics without duplicating
		// credential-derived bytes in the UI log.
		s.emit(StateStarting, "debug", fmt.Sprintf("已发送 EAPOL 报文（%d 字节，敏感载荷已隐藏）", len(frame)))
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.handle.WritePacketData(frame)
}

func (s *Session) Close() {
	s.closeOnce.Do(func() { s.handle.Close() })
}

func (s *Session) clearCredentials() {
	s.cfg.Password = ""
}

func cloneMAC(value net.HardwareAddr) net.HardwareAddr {
	cloned := make(net.HardwareAddr, len(value))
	copy(cloned, value)
	return cloned
}
