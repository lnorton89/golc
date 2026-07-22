// packet_test.go proves ARTN-03's byte-exact ArtDMX packet contract
// (04-01-PLAN.md, Task 2): a golden byte vector for EncodeArtDMX, the
// three data-length rejection cases, PortAddress distinctness across
// sample universes, and a long-run assertion that the sequence helper
// never yields 0.
//
// This is an internal test package (package artnet, not artnet_test)
// because it exercises the unexported nextSeq helper alongside the
// exported EncodeArtDMX/PortAddress functions.
package artnet

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"
)

// TestEncodeArtDMXGoldenVector asserts the exact 18+N byte layout: id,
// little-endian OpCode, protocol version, seq, physical, packed
// Sub-Net/Universe then Net, big-endian length, then data.
func TestEncodeArtDMXGoldenVector(t *testing.T) {
	// universe 17 -> Sub-Net=1, Universe=1 -> portAddress low byte 0x11,
	// Net=0 -> high byte 0x00.
	portAddress := PortAddress(17)
	data := []byte{0x01, 0x02, 0x03, 0x04}

	got, err := EncodeArtDMX(1, 0, portAddress, data)
	if err != nil {
		t.Fatalf("EncodeArtDMX failed: %v", err)
	}

	want := []byte{
		'A', 'r', 't', '-', 'N', 'e', 't', 0x00, // ID
		0x00, 0x50, // OpCode little-endian (0x5000)
		0x00,       // ProtVerHi
		0x0e,       // ProtVerLo (protocol version 14)
		0x01,       // Sequence
		0x00,       // Physical
		0x11,       // SubUni: Sub-Net=1, Universe=1
		0x00,       // Net=0
		0x00, 0x04, // Length big-endian (4)
		0x01, 0x02, 0x03, 0x04, // data
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d bytes, got %d: % x", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d: expected 0x%02x, got 0x%02x\nwant: % x\ngot:  % x", i, want[i], got[i], want, got)
		}
	}
}

func TestEncodeArtDMXLengthRejections(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{name: "odd length", data: []byte{0x01, 0x02, 0x03}},
		{name: "too short", data: []byte{0x01}},
		{name: "too long", data: make([]byte, 513)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := EncodeArtDMX(1, 0, PortAddress(1), testCase.data)
			if err == nil {
				t.Fatalf("expected %s to be rejected", testCase.name)
			}
			if !strings.Contains(err.Error(), "GOLC_ARTNET_DMX_LENGTH_INVALID") {
				t.Fatalf("expected GOLC_ARTNET_DMX_LENGTH_INVALID, got %v", err)
			}
		})
	}
}

// TestPortAddressDistinct proves PortAddress packs Net=0, Sub-Net=(universe
// >>4)&0xF, Universe=universe&0xF and produces distinct Port-Addresses
// across the 1..64 universe range this project's own maxUniverseSearch
// ceiling covers.
func TestPortAddressDistinct(t *testing.T) {
	seen := map[uint16]int{}
	for universe := 1; universe <= 64; universe++ {
		pa := PortAddress(universe)
		if existing, ok := seen[pa]; ok {
			t.Fatalf("universe %d produced Port-Address 0x%04x, already used by universe %d", universe, pa, existing)
		}
		seen[pa] = universe
	}
}

func TestPortAddressPacking(t *testing.T) {
	cases := []struct {
		universe int
		want     uint16
	}{
		{universe: 1, want: 0x0001},
		{universe: 16, want: 0x0010},
		{universe: 17, want: 0x0011},
		{universe: 64, want: 0x0040},
	}
	for _, testCase := range cases {
		got := PortAddress(testCase.universe)
		if got != testCase.want {
			t.Fatalf("PortAddress(%d) = 0x%04x, want 0x%04x", testCase.universe, got, testCase.want)
		}
	}
}

// TestSequenceNeverZero proves the sequence helper cycles 1->255->1 and
// never returns 0 across a long simulated run (Pitfall 2: sequence 0
// disables receiver reordering).
func TestSequenceNeverZero(t *testing.T) {
	seq := uint8(0)
	for i := 0; i < 1024; i++ {
		seq = nextSeq(seq)
		if seq == 0 {
			t.Fatalf("nextSeq produced 0 at iteration %d", i)
		}
	}
}

func TestSequenceWrap(t *testing.T) {
	seq := nextSeq(0)
	if seq != 1 {
		t.Fatalf("expected the first sequence value to be 1, got %d", seq)
	}
	seq = uint8(254)
	seq = nextSeq(seq)
	if seq != 255 {
		t.Fatalf("expected nextSeq(254) == 255, got %d", seq)
	}
	seq = nextSeq(seq)
	if seq != 1 {
		t.Fatalf("expected nextSeq(255) to wrap to 1, got %d", seq)
	}
}

// buildGoodArtPollReply constructs a spec-shaped, artPollReplyMinLen-byte
// ArtPollReply body carrying ip/shortName/longName and one declared
// output port at (netSwitch, subSwitch, swOut) -- the "known-good"
// vector this file's own decode tests and discovery_test.go's Discover
// tests both exercise.
func buildGoodArtPollReply(ip [4]byte, shortName, longName string, netSwitch, subSwitch, swOut byte) []byte {
	buf := make([]byte, artPollReplyMinLen)
	copy(buf[0:8], []byte("Art-Net\x00"))
	binary.LittleEndian.PutUint16(buf[8:10], opPollReply)
	copy(buf[10:14], ip[:])
	binary.LittleEndian.PutUint16(buf[14:16], artNetPort)
	buf[18] = netSwitch
	buf[19] = subSwitch
	copy(buf[26:44], []byte(shortName))
	copy(buf[44:108], []byte(longName))
	buf[173] = 1 // NumPortsLo: one declared output port
	buf[190] = swOut
	return buf
}

// TestEncodeArtPollGoldenVector asserts the exact 14-byte ArtPoll layout:
// id, little-endian OpPoll opcode, protocol version, TalkToMe, Priority.
func TestArtPollEncodeGoldenVector(t *testing.T) {
	got := EncodeArtPoll()
	want := []byte{
		'A', 'r', 't', '-', 'N', 'e', 't', 0x00, // ID
		0x00, 0x20, // OpCode little-endian (0x2000)
		0x00, // ProtVerHi
		0x0e, // ProtVerLo
		0x00, // TalkToMe
		0x00, // Priority (DpAll)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d bytes, got %d: % x", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d: expected 0x%02x, got 0x%02x\nwant: % x\ngot:  % x", i, want[i], got[i], want, got)
		}
	}
}

// TestDecodeArtPollReplyGoodVector proves DecodeArtPollReply parses a
// known-good reply's IP, short/long name, and Port-Address fields.
func TestArtPollReplyDecodeGoodVector(t *testing.T) {
	buf := buildGoodArtPollReply([4]byte{10, 0, 0, 5}, "GOLC-Node", "GOLC Test Node Long Name", 0x00, 0x01, 0x03)

	reply, err := DecodeArtPollReply(buf)
	if err != nil {
		t.Fatalf("DecodeArtPollReply failed: %v", err)
	}
	if !reply.IP.Equal(net.IPv4(10, 0, 0, 5)) {
		t.Fatalf("expected IP 10.0.0.5, got %v", reply.IP)
	}
	if reply.ShortName != "GOLC-Node" {
		t.Fatalf("expected short name %q, got %q", "GOLC-Node", reply.ShortName)
	}
	if reply.LongName != "GOLC Test Node Long Name" {
		t.Fatalf("expected long name %q, got %q", "GOLC Test Node Long Name", reply.LongName)
	}
	if len(reply.PortAddresses) != 1 {
		t.Fatalf("expected exactly 1 port address, got %d: %v", len(reply.PortAddresses), reply.PortAddresses)
	}
	// netSwitch=0x00 -> Net=0; subSwitch=0x01 -> Sub-Net=1; swOut=0x03 ->
	// Universe=3 -> Port-Address 0x0013.
	if reply.PortAddresses[0] != 0x0013 {
		t.Fatalf("expected port address 0x0013, got 0x%04x", reply.PortAddresses[0])
	}
}

// TestDecodeArtPollReplyMalformed proves every malformed-input case
// (empty, short header, wrong id, wrong opcode, oversized declared port
// count) returns GOLC_ARTNET_POLLREPLY_INVALID without panicking
// (Security Domain V5, T-04-01).
func TestArtPollReplyDecodeMalformed(t *testing.T) {
	good := buildGoodArtPollReply([4]byte{10, 0, 0, 5}, "N", "L", 0x00, 0x01, 0x03)

	wrongID := append([]byte(nil), good...)
	wrongID[0] = 'X'

	wrongOpcode := append([]byte(nil), good...)
	binary.LittleEndian.PutUint16(wrongOpcode[8:10], opPoll)

	oversizedPortCount := append([]byte(nil), good...)
	oversizedPortCount[173] = artPollReplyMaxPorts + 1

	cases := []struct {
		name string
		buf  []byte
	}{
		{name: "empty", buf: nil},
		{name: "short header", buf: good[:artPollReplyMinLen-1]},
		{name: "wrong id", buf: wrongID},
		{name: "wrong opcode", buf: wrongOpcode},
		{name: "oversized declared port count", buf: oversizedPortCount},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DecodeArtPollReply(testCase.buf)
			if err == nil {
				t.Fatalf("expected %s to be rejected", testCase.name)
			}
			if !strings.Contains(err.Error(), "GOLC_ARTNET_POLLREPLY_INVALID") {
				t.Fatalf("expected GOLC_ARTNET_POLLREPLY_INVALID, got %v", err)
			}
		})
	}
}
