// protocol_test.go covers internal/script/protocol.go (08-05-PLAN.md
// Task 1): a table-driven test over every <behavior> bullet, plus a
// round-trip encode/decode case for every declared frame kind.
package script

import (
	"bytes"
	"strings"
	"testing"
)

func TestFrameDecodeCmdCall(t *testing.T) {
	line := []byte(`{"kind":"cmd-call","id":"c1","method":"scene.activate","params":{"name":"Alpha"}}`)

	frame, err := DecodeFrame(line)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	call, ok := frame.(CmdCallFrame)
	if !ok {
		t.Fatalf("expected CmdCallFrame, got %T", frame)
	}
	if call.FrameKind() != FrameKindCmdCall {
		t.Fatalf("FrameKind() = %q, want %q", call.FrameKind(), FrameKindCmdCall)
	}
	if call.ID != "c1" {
		t.Fatalf("ID = %q, want %q", call.ID, "c1")
	}
	if call.Method != "scene.activate" {
		t.Fatalf("Method = %q, want %q", call.Method, "scene.activate")
	}
	if !bytes.Contains(call.Params, []byte(`"name":"Alpha"`)) {
		t.Fatalf("Params = %s, want it to contain name:Alpha", call.Params)
	}
}

func TestFrameDecodeUnknownKind(t *testing.T) {
	line := []byte(`{"kind":"bogus-kind","id":"c1"}`)

	_, err := DecodeFrame(line)
	if err == nil {
		t.Fatal("expected an error for an unknown frame kind")
	}
	if !strings.Contains(err.Error(), "GOLC_SCRIPT_FRAME_UNKNOWN") {
		t.Fatalf("expected GOLC_SCRIPT_FRAME_UNKNOWN, got %v", err)
	}
	if !strings.Contains(err.Error(), "bogus-kind") {
		t.Fatalf("expected the error to name the offending kind, got %v", err)
	}
}

func TestFrameDecodeMalformedJSON(t *testing.T) {
	_, err := DecodeFrame([]byte(`{"kind":"cmd-call", not json`))
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "GOLC_SCRIPT_FRAME_MALFORMED") {
		t.Fatalf("expected GOLC_SCRIPT_FRAME_MALFORMED, got %v", err)
	}
}

func TestFrameDecodeMissingKind(t *testing.T) {
	_, err := DecodeFrame([]byte(`{"id":"c1"}`))
	if err == nil {
		t.Fatal("expected an error for a missing kind field")
	}
	if !strings.Contains(err.Error(), "GOLC_SCRIPT_FRAME_UNKNOWN") {
		t.Fatalf("expected GOLC_SCRIPT_FRAME_UNKNOWN for a blank kind, got %v", err)
	}
}

func TestFrameOversizedLineFailsWithoutBufferingWhole(t *testing.T) {
	// A line well beyond maxFrameBytes: scanFrameLine must fail with
	// GOLC_SCRIPT_FRAME_TOO_LARGE, and the bounded bufio.Scanner (Buffer
	// capped at maxFrameBytes, per newFrameReader) structurally never
	// grows its internal buffer past that cap to hold it.
	oversized := bytes.Repeat([]byte("a"), maxFrameBytes+1024)
	oversized = append(oversized, '\n')

	scanner := newFrameReader(bytes.NewReader(oversized))
	_, ok, err := scanFrameLine(scanner)
	if ok {
		t.Fatal("expected scanFrameLine to report !ok for an oversized line")
	}
	if err == nil {
		t.Fatal("expected an error for an oversized line")
	}
	if !strings.Contains(err.Error(), "GOLC_SCRIPT_FRAME_TOO_LARGE") {
		t.Fatalf("expected GOLC_SCRIPT_FRAME_TOO_LARGE, got %v", err)
	}
}

func TestFrameEncodeCmdResultSingleLineNoEmbeddedNewline(t *testing.T) {
	var buf bytes.Buffer
	frame := CmdResultFrame{ID: "c1", Ok: true, Result: []byte(`{"ok":true}`)}
	if err := EncodeFrame(&buf, frame); err != nil {
		t.Fatalf("EncodeFrame: %v", err)
	}

	encoded := buf.String()
	if strings.Count(encoded, "\n") != 1 {
		t.Fatalf("expected exactly one trailing newline, got %d in %q", strings.Count(encoded, "\n"), encoded)
	}
	if !strings.HasSuffix(encoded, "\n") {
		t.Fatalf("expected the encoded frame to end with a newline, got %q", encoded)
	}
	if strings.Contains(strings.TrimSuffix(encoded, "\n"), "\n") {
		t.Fatalf("expected no embedded newline before the trailing one, got %q", encoded)
	}
}

func TestFrameEncodeCmdResultErrorNeverCarriesPartialResult(t *testing.T) {
	var buf bytes.Buffer
	frame := CmdResultFrame{ID: "c1", Ok: false, Code: "GOLC_SCRIPT_METHOD_UNKNOWN", Message: "unknown method"}
	if err := EncodeFrame(&buf, frame); err != nil {
		t.Fatalf("EncodeFrame: %v", err)
	}

	decoded, err := DecodeFrame(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	result, ok := decoded.(CmdResultFrame)
	if !ok {
		t.Fatalf("expected CmdResultFrame, got %T", decoded)
	}
	if result.Ok {
		t.Fatal("expected Ok:false for an error result")
	}
	if result.Code != "GOLC_SCRIPT_METHOD_UNKNOWN" || result.Message != "unknown method" {
		t.Fatalf("expected Code/Message to round-trip, got Code=%q Message=%q", result.Code, result.Message)
	}
	if len(result.Result) != 0 {
		t.Fatalf("expected an error result to carry no Result payload, got %s", result.Result)
	}
}

// TestFrameRoundTrip encodes then decodes every declared frame kind and
// asserts the decoded value's FrameKind matches, proving the encoder and
// decoder agree on every kind this package declares.
func TestFrameRoundTrip(t *testing.T) {
	cases := []Frame{
		CmdCallFrame{ID: "1", Method: "scene.activate", Params: []byte(`{"name":"Alpha"}`)},
		LogFrame{Level: "info", Message: "hello", Source: "script.ts:3"},
		ReadyFrame{},
		DoneFrame{ExitReason: "completed"},
		CmdResultFrame{ID: "1", Ok: true, Result: []byte(`{"x":1}`)},
		CancelFrame{Reason: "stop requested"},
	}

	for _, original := range cases {
		t.Run(original.FrameKind(), func(t *testing.T) {
			var buf bytes.Buffer
			if err := EncodeFrame(&buf, original); err != nil {
				t.Fatalf("EncodeFrame: %v", err)
			}
			line := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))

			decoded, err := DecodeFrame(line)
			if err != nil {
				t.Fatalf("DecodeFrame: %v", err)
			}
			if decoded.FrameKind() != original.FrameKind() {
				t.Fatalf("FrameKind() = %q, want %q", decoded.FrameKind(), original.FrameKind())
			}
		})
	}
}
