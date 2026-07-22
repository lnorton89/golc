// packet.go implements ARTN-03's byte-exact Art-Net 4 ArtDMX packet
// encoder (04-01-PLAN.md Task 2, 04-RESEARCH.md Pattern 1): a hand-rolled,
// spec-driven encoder rather than a third-party Art-Net library (see
// 04-RESEARCH.md's Package Legitimacy Audit -- every realistic candidate
// found during research is either self-described "unstable" or carries a
// zero-adoption risk profile unsuitable for this correctness-critical
// send path). PortAddress implements Assumption A1's locked mapping of
// GOLC's flat deployment.Instance.Universe integer onto Art-Net's 15-bit
// Net/Sub-Net/Universe Port-Address, and the sequence-wrap helper avoids
// Pitfall 2 (sequence 0 disables receiver reordering) by cycling
// 1->255->1 and never emitting 0.
package artnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

// Art-Net 4 wire constants (04-RESEARCH.md Pattern 1, cross-checked
// art-net.org.uk / Wikipedia Art-Net summary).
const (
	// artNetPort is the fixed Art-Net UDP port (0x1936).
	artNetPort = 6454
	// opOutputDMX is the ArtDMX OpCode, written little-endian on the wire.
	opOutputDMX = 0x5000
	// protVerHi/protVerLo are the fixed Art-Net protocol version bytes;
	// protVerLo=0x0e is protocol version 14.
	protVerHi = 0x00
	protVerLo = 0x0e
)

// EncodeArtDMX builds one byte-exact ArtDMX packet: "Art-Net\0" (8 bytes),
// little-endian OpCode, protocol version, seq, physical, the 15-bit
// portAddress packed as byte14=SubNet<<4|Universe / byte15=Net, a
// big-endian data length, then data itself.
//
// seq is 1..255 (see nextSeq -- 0x00 disables sequence checking on the
// receiving node per spec and must never be sent once sequencing is
// enabled, Pitfall 2). data's length must be even and within [2,512]
// (GOLC_ARTNET_DMX_LENGTH_INVALID otherwise) -- DMX values are 8-bit
// bytes, not runes/code points.
func EncodeArtDMX(seq, physical uint8, portAddress uint16, data []byte) ([]byte, error) {
	if len(data) < 2 || len(data) > 512 || len(data)%2 != 0 {
		return nil, fmt.Errorf("GOLC_ARTNET_DMX_LENGTH_INVALID: length %d must be even and in [2,512]", len(data))
	}

	buf := make([]byte, 18+len(data))
	copy(buf[0:8], []byte("Art-Net\x00"))
	binary.LittleEndian.PutUint16(buf[8:10], opOutputDMX)
	buf[10] = protVerHi
	buf[11] = protVerLo
	buf[12] = seq
	buf[13] = physical
	buf[14] = byte(portAddress & 0xff)        // SubUni: low nibble Sub-Net, high nibble Universe
	buf[15] = byte((portAddress >> 8) & 0x7f) // Net: top 7 bits, bit 7 reserved/zero
	binary.BigEndian.PutUint16(buf[16:18], uint16(len(data)))
	copy(buf[18:], data)
	return buf, nil
}

// PortAddress implements the phase-locked mapping (04-RESEARCH.md
// Assumption A1, 04-PATTERNS.md Pitfall 3) of GOLC's flat
// deployment.Instance.Universe integer onto Art-Net's 15-bit Port-Address:
// Net=0 (fixed), Sub-Net=(universe>>4)&0xF, Universe=universe&0xF. This is
// a named, tested function rather than an inline expression precisely
// because the mapping is a locked design decision, not an incidental
// detail (Pitfall 3's own warning sign).
func PortAddress(universe int) uint16 {
	subNet := uint16(universe>>4) & 0xF
	universeBits := uint16(universe) & 0xF
	net := uint16(0) & 0x7F
	return (net << 8) | (subNet << 4) | universeBits
}

// nextSeq returns the next Art-Net sequence value after prev, wrapping
// 1->255->1 and never returning 0 (Pitfall 2: sequence 0 disables
// receiver reordering). Passing 0 (the zero value, e.g. before the first
// packet) returns 1, so the first packet sent always starts the sequence
// at 1.
func nextSeq(prev uint8) uint8 {
	if prev == 0 || prev == 255 {
		return 1
	}
	return prev + 1
}

// Art-Net 4 ArtPoll/ArtPollReply opcodes (04-06-PLAN.md Task 1,
// 04-RESEARCH.md Pattern 4; cross-checked art-net.org.uk against
// jsimonetti/go-artnet/packet/code/opcode.go's OpPoll=0x2000/
// OpPollReply=0x2100 constants).
const (
	opPoll      = 0x2000
	opPollReply = 0x2100
)

// artPollReplyMinLen is the minimum ArtPollReply body size this decoder
// requires before it will read any fixed-offset field: id(8) + opcode(2)
// + ip(4) + port(2) + versInfoH/L(2) + netSwitch/subSwitch(2) + oem(2) +
// ubeaVersion(1) + status1(1) + estaMan(2) + shortName(18) + longName(64)
// + nodeReport(64) + numPortsHi/Lo(2) + portTypes(4) + goodInput(4) +
// goodOutput(4) + swIn(4) + swOut(4) + swVideo/Macro/Remote(3) + spare(3)
// + style(1) + mac(6) + bindIP(4) + bindIndex(1) + status2(1) +
// filler(26) = 239 bytes (Security Domain V5: a buffer shorter than this
// is rejected before any fixed-offset field is read, never silently
// zero-padded or partially parsed).
const artPollReplyMinLen = 239

// artPollReplyMaxPorts is the hard ceiling ArtPollReply's declared port
// count (NumPortsLo, byte offset 173) must never exceed: a real Art-Net
// node reports at most 4 physical DMX ports per reply (Security Domain
// V5/T-04-01). A declared count above this is rejected as
// GOLC_ARTNET_POLLREPLY_INVALID rather than used to index the
// fixed-4-element SwOut array, which would otherwise read past its own
// bounds.
const artPollReplyMaxPorts = 4

// ArtPollReply is the subset of an inbound ArtPollReply this project
// needs (04-06-PLAN.md): the node's IP, its short/long name, and every
// Port-Address it reports output for (one per declared SwOut port entry,
// combining NetSwitch/SubSwitch/SwOut into the same Net<<8|SubNet<<4|
// Universe packing PortAddress uses).
type ArtPollReply struct {
	IP            net.IP
	ShortName     string
	LongName      string
	PortAddresses []uint16
}

// EncodeArtPoll builds a spec-shaped ArtPoll packet (04-RESEARCH.md
// Pattern 4): id + little-endian opPoll opcode + protocol version, then
// TalkToMe=0x00 (no reply-on-change/diagnostics -- this is a single
// bounded scan, not a persistent subscription) and Priority=0x00 (DpAll,
// no diagnostic-severity filtering). This ArtPoll broadcast is the
// opt-in discovery scan (CONTEXT D-06); it does not conflict with D-07's
// no-broadcast rule for the live DMX *output* path.
func EncodeArtPoll() []byte {
	buf := make([]byte, 14)
	copy(buf[0:8], []byte("Art-Net\x00"))
	binary.LittleEndian.PutUint16(buf[8:10], opPoll)
	buf[10] = protVerHi
	buf[11] = protVerLo
	buf[12] = 0x00 // TalkToMe
	buf[13] = 0x00 // Priority (DpAll)
	return buf
}

// artPollReplyNullTerminated returns field up to (not including) its
// first 0x00 byte, or the whole field if it contains none. field is
// always a fixed-length slice of the already length-validated buf, so
// this can never read out of bounds regardless of the reply's content
// (Security Domain V5).
func artPollReplyNullTerminated(field []byte) string {
	if idx := bytes.IndexByte(field, 0); idx >= 0 {
		return string(field[:idx])
	}
	return string(field)
}

// DecodeArtPollReply strictly decodes buf into an ArtPollReply,
// bounds-checking every length/count field against a hard ceiling before
// it is used to index or allocate (Security Domain V5, T-04-01): buf
// shorter than artPollReplyMinLen, a missing/malformed "Art-Net\0" id, a
// non-ArtPollReply opcode, or a declared port count exceeding
// artPollReplyMaxPorts each return GOLC_ARTNET_POLLREPLY_INVALID rather
// than a panic or an out-of-range read -- this is untrusted network
// input from an arbitrary (possibly spoofed) device.
func DecodeArtPollReply(buf []byte) (ArtPollReply, error) {
	if len(buf) < artPollReplyMinLen {
		return ArtPollReply{}, fmt.Errorf(
			"GOLC_ARTNET_POLLREPLY_INVALID: buffer length %d is shorter than the minimum ArtPollReply size %d", len(buf), artPollReplyMinLen)
	}
	if !bytes.Equal(buf[0:8], []byte("Art-Net\x00")) {
		return ArtPollReply{}, fmt.Errorf("GOLC_ARTNET_POLLREPLY_INVALID: missing or malformed \"Art-Net\\0\" id")
	}
	opcode := binary.LittleEndian.Uint16(buf[8:10])
	if opcode != opPollReply {
		return ArtPollReply{}, fmt.Errorf(
			"GOLC_ARTNET_POLLREPLY_INVALID: opcode 0x%04x is not ArtPollReply (0x%04x)", opcode, opPollReply)
	}

	ip := net.IPv4(buf[10], buf[11], buf[12], buf[13])
	netSwitch := buf[18]
	subSwitch := buf[19]
	shortName := artPollReplyNullTerminated(buf[26:44])
	longName := artPollReplyNullTerminated(buf[44:108])

	numPorts := int(buf[173])
	if numPorts < 0 || numPorts > artPollReplyMaxPorts {
		return ArtPollReply{}, fmt.Errorf(
			"GOLC_ARTNET_POLLREPLY_INVALID: declared port count %d exceeds the hard ceiling of %d", numPorts, artPollReplyMaxPorts)
	}

	swOut := buf[190:194]
	net7 := uint16(netSwitch) & 0x7F
	subNet4 := uint16(subSwitch) & 0xF
	portAddresses := make([]uint16, 0, numPorts)
	for i := 0; i < numPorts; i++ {
		universeBits := uint16(swOut[i]) & 0xF
		portAddresses = append(portAddresses, (net7<<8)|(subNet4<<4)|universeBits)
	}

	return ArtPollReply{
		IP:            ip,
		ShortName:     shortName,
		LongName:      longName,
		PortAddresses: portAddresses,
	}, nil
}
