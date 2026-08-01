// protocol_test.go covers internal/script/protocol.go (08-05-PLAN.md
// Task 1): a table-driven test over every <behavior> bullet, plus a
// round-trip encode/decode case for every declared frame kind.
package script

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrameDecodeCmdCall(t *testing.T) {
	line := []byte(`{"kind":"cmd-call","id":"c1","method":"scene.activate","params":{"name":"Alpha"}}`)

	frame, err := DecodeFrame(line)
	require.NoError(t, err, "DecodeFrame")
	call, ok := frame.(CmdCallFrame)
	require.True(t, ok, "expected CmdCallFrame, got %T", frame)
	require.Equal(t, FrameKindCmdCall, call.FrameKind(), "FrameKind() = %q, want %q", call.FrameKind(), FrameKindCmdCall)
	require.Equal(t, "c1", call.ID, "ID = %q, want %q", call.ID, "c1")
	require.Equal(t, "scene.activate", call.Method, "Method = %q, want %q", call.Method, "scene.activate")
	require.Contains(t, string(call.Params), `"name":"Alpha"`, "Params = %s, want it to contain name:Alpha", call.Params)
}

func TestFrameDecodeUnknownKind(t *testing.T) {
	line := []byte(`{"kind":"bogus-kind","id":"c1"}`)

	_, err := DecodeFrame(line)
	require.Error(t, err, "expected an error for an unknown frame kind")
	require.Contains(t, err.Error(), "GOLC_SCRIPT_FRAME_UNKNOWN", "expected GOLC_SCRIPT_FRAME_UNKNOWN, got %v", err)
	require.Contains(t, err.Error(), "bogus-kind", "expected the error to name the offending kind, got %v", err)
}

func TestFrameDecodeMalformedJSON(t *testing.T) {
	_, err := DecodeFrame([]byte(`{"kind":"cmd-call", not json`))
	require.Error(t, err, "expected an error for malformed JSON")
	require.Contains(t, err.Error(), "GOLC_SCRIPT_FRAME_MALFORMED", "expected GOLC_SCRIPT_FRAME_MALFORMED, got %v", err)
}

func TestFrameDecodeMissingKind(t *testing.T) {
	_, err := DecodeFrame([]byte(`{"id":"c1"}`))
	require.Error(t, err, "expected an error for a missing kind field")
	require.Contains(t, err.Error(), "GOLC_SCRIPT_FRAME_UNKNOWN", "expected GOLC_SCRIPT_FRAME_UNKNOWN for a blank kind, got %v", err)
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
	require.False(t, ok, "expected scanFrameLine to report !ok for an oversized line")
	require.Error(t, err, "expected an error for an oversized line")
	require.Contains(t, err.Error(), "GOLC_SCRIPT_FRAME_TOO_LARGE", "expected GOLC_SCRIPT_FRAME_TOO_LARGE, got %v", err)
}

func TestFrameEncodeCmdResultSingleLineNoEmbeddedNewline(t *testing.T) {
	var buf bytes.Buffer
	frame := CmdResultFrame{ID: "c1", Ok: true, Result: []byte(`{"ok":true}`)}
	require.NoError(t, EncodeFrame(&buf, frame), "EncodeFrame")

	encoded := buf.String()
	require.Equal(t, 1, strings.Count(encoded, "\n"), "expected exactly one trailing newline, got %d in %q", strings.Count(encoded, "\n"), encoded)
	require.True(t, strings.HasSuffix(encoded, "\n"), "expected the encoded frame to end with a newline, got %q", encoded)
	require.False(t, strings.Contains(strings.TrimSuffix(encoded, "\n"), "\n"), "expected no embedded newline before the trailing one, got %q", encoded)
}

func TestFrameEncodeCmdResultErrorNeverCarriesPartialResult(t *testing.T) {
	var buf bytes.Buffer
	frame := CmdResultFrame{ID: "c1", Ok: false, Code: "GOLC_SCRIPT_METHOD_UNKNOWN", Message: "unknown method"}
	require.NoError(t, EncodeFrame(&buf, frame), "EncodeFrame")

	decoded, err := DecodeFrame(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	require.NoError(t, err, "DecodeFrame")
	result, ok := decoded.(CmdResultFrame)
	require.True(t, ok, "expected CmdResultFrame, got %T", decoded)
	require.False(t, result.Ok, "expected Ok:false for an error result")
	require.Equal(t, "GOLC_SCRIPT_METHOD_UNKNOWN", result.Code, "expected Code/Message to round-trip, got Code=%q Message=%q", result.Code, result.Message)
	require.Equal(t, "unknown method", result.Message, "expected Code/Message to round-trip, got Code=%q Message=%q", result.Code, result.Message)
	require.Empty(t, result.Result, "expected an error result to carry no Result payload, got %s", result.Result)
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
			require.NoError(t, EncodeFrame(&buf, original), "EncodeFrame")
			line := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))

			decoded, err := DecodeFrame(line)
			require.NoError(t, err, "DecodeFrame")
			require.Equal(t, original.FrameKind(), decoded.FrameKind(), "FrameKind() = %q, want %q", decoded.FrameKind(), original.FrameKind())
		})
	}
}
