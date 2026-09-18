package networkdiag

const (
	ipSuccess             = 0
	ipDestNetUnreachable  = 11002
	ipDestHostUnreachable = 11003
	ipDestProtUnreachable = 11004
	ipDestPortUnreachable = 11005
	ipPacketTooBig        = 11009
	ipRequestTimedOut     = 11010
	ipTTLExpiredTransit   = 11013
)

type icmpProbeResult struct {
	Status        uint32
	Address       string
	RoundTripTime uint32
}
