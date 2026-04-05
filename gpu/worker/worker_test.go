package worker

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestProtocolStructSizes verifies that the fixed-layout request/response
// structs have the expected byte sizes.  The sizes must match the C struct
// definitions in gpu/server/protocol.h exactly; any mismatch will cause
// silent data corruption.
func TestProtocolStructSizes(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want int
	}{
		{"reqHeader", reqHeader{}, 8},
		{"respHeader", respHeader{}, 8},
		{"ShmName", ShmName{}, 64},
		{"allocGridReq", allocGridReq{}, 12},
		{"allocGridResp", allocGridResp{}, 8},
		{"freeGridReq", freeGridReq{}, 8},
		{"uploadFieldsReq", uploadFieldsReq{}, 72},
		{"runFDTDReq", runFDTDReq{}, 96},
		{"allocBVHReq", allocBVHReq{}, 72},
		{"allocBVHResp", allocBVHResp{}, 8},
		{"freeBVHReq", freeBVHReq{}, 8},
		{"traceRaysReq", traceRaysReq{}, 140},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := binary.Write(&buf, binary.LittleEndian, tt.val); err != nil {
				t.Fatalf("binary.Write: %v", err)
			}
			if got := buf.Len(); got != tt.want {
				t.Errorf("size = %d bytes, want %d", got, tt.want)
			}
		})
	}
}

// TestShmNameRoundTrip verifies shmNameOf and ShmName.String.
func TestShmNameRoundTrip(t *testing.T) {
	cases := []string{
		"algo_gpu_deadbeef01234567",
		"",
		"short",
	}
	for _, c := range cases {
		n := shmNameOf(c)
		got := n.String()
		if got != c {
			t.Errorf("ShmName round-trip %q → %q", c, got)
		}
	}
}

// TestShmNameTruncation verifies that names longer than 63 bytes are truncated
// and the array is always NUL-terminated.
func TestShmNameTruncation(t *testing.T) {
	long := string(make([]byte, 100))
	for i := range []byte(long) {
		long = long[:i] + "a" + long[i+1:]
	}
	long = "a100chars" + long[:91]
	n := shmNameOf(long)
	if n[63] != 0 {
		t.Error("last byte of ShmName should be NUL after truncation")
	}
}

// TestReqHeaderRoundTrip encodes and decodes a request header.
func TestReqHeaderRoundTrip(t *testing.T) {
	h := reqHeader{Type: MsgRunFDTD, Flags: 0, PayloadLen: 92}
	var buf bytes.Buffer
	if err := h.writeTo(&buf); err != nil {
		t.Fatalf("writeTo: %v", err)
	}
	if buf.Len() != 8 {
		t.Fatalf("header size %d, want 8", buf.Len())
	}
	var back reqHeader
	if err := binary.Read(&buf, byteOrder, &back); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if back != h {
		t.Errorf("got %+v, want %+v", back, h)
	}
}

// TestRespHeaderRoundTrip encodes and decodes a response header.
func TestRespHeaderRoundTrip(t *testing.T) {
	h := respHeader{Status: StatusOK, ResponseLen: 8}
	var buf bytes.Buffer
	if err := binary.Write(&buf, byteOrder, h); err != nil {
		t.Fatalf("Write: %v", err)
	}
	back, err := readRespHeader(&buf)
	if err != nil {
		t.Fatalf("readRespHeader: %v", err)
	}
	if back != h {
		t.Errorf("got %+v, want %+v", back, h)
	}
}

// TestServerError formats error messages correctly.
func TestServerError(t *testing.T) {
	cases := []struct {
		code uint32
		want string
	}{
		{StatusNotImpl, "not implemented"},
		{StatusBadHandle, "bad handle"},
		{StatusOOM, "GPU out of memory"},
		{StatusCUDA, "CUDA error"},
		{StatusBadMsg, "bad message"},
		{99, "code 99"},
	}
	for _, c := range cases {
		err := &ServerError{Code: c.code, MsgType: MsgPing}
		if got := err.Error(); got == "" {
			t.Errorf("code %d: empty error string", c.code)
		}
		if !contains(err.Error(), c.want) {
			t.Errorf("code %d: %q does not contain %q", c.code, err.Error(), c.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) &&
		func() bool {
			for i := range s {
				if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
