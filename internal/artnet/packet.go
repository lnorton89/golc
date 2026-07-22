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
	"encoding/binary"
	"fmt"
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
