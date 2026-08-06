package worker

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Worker manages a long-lived GPU server subprocess and serialises all
// socket I/O through a single goroutine.
//
// Typical usage:
//
//	w, err := Start(ctx, "/path/to/algo-acoustics-gpu")
//	defer w.Close()
//	if err := w.Ping(); err != nil { ... }
//	handle, err := w.AllocGrid(ctx, 128, 128, 128)
type Worker struct {
	cmd    *exec.Cmd
	conn   net.Conn
	queue  chan call
	closed chan struct{}
}

type call struct {
	msgType uint16
	payload []byte
	result  chan callResult
}

type callResult struct {
	data []byte
	err  error
}

// Start launches the GPU server binary at serverBin and connects to it.
// The binary must accept a single argument: the Unix socket path to listen on.
// ctx is used only for the initial connection; the worker runs until Close.
func Start(ctx context.Context, serverBin string) (*Worker, error) {
	// Choose a temporary socket path.
	sockPath := filepath.Join(os.TempDir(),
		fmt.Sprintf("algo_gpu_%d.sock", os.Getpid()))
	os.Remove(sockPath)

	// serverBin is an executable path resolved by Probe/StartIfAvailable or
	// explicitly supplied by the caller; no shell interpolation is involved.
	cmd := exec.CommandContext(ctx, serverBin, "--socket", sockPath) // #nosec G702
	cmd.Stdout = os.Stderr                                           // server log → caller's stderr
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("start GPU server %q: %w", serverBin, err)
	}

	// Wait for the server to create the socket (up to 5 s).
	conn, err := dialUnix(sockPath, 5*time.Second)
	if err != nil {
		cmd.Process.Kill() //nolint:errcheck
		return nil, fmt.Errorf("connect to GPU server: %w", err)
	}

	w := &Worker{
		cmd:    cmd,
		conn:   conn,
		queue:  make(chan call, 64),
		closed: make(chan struct{}),
	}
	go w.loop()

	return w, nil
}

// Ping sends a no-op message and returns an error if the server is not
// responsive.  Useful for health checks.
func (w *Worker) Ping(ctx context.Context) error {
	_, err := w.do(ctx, MsgPing, nil)
	return err
}

// Close sends a shutdown message, waits for the subprocess to exit, and
// releases the connection.
func (w *Worker) Close() error {
	// Best-effort shutdown; ignore errors if already closed.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, _ = w.do(ctx, MsgShutdown, nil)

	close(w.closed)
	w.conn.Close()

	if w.cmd.Process != nil {
		_ = w.cmd.Wait()
	}

	return nil
}

// do submits one request and blocks until the response arrives.
// It is safe to call do from multiple goroutines; the worker goroutine
// serialises them.
func (w *Worker) do(ctx context.Context, msgType uint16, payload []byte) ([]byte, error) {
	result := make(chan callResult, 1)

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	case <-w.closed:
		return nil, errors.New("worker closed")
	case w.queue <- call{msgType: msgType, payload: payload, result: result}:
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	case r := <-result:
		return r.data, r.err
	}
}

// loop is the single goroutine that owns the socket connection.
// All other goroutines submit work via w.queue.
func (w *Worker) loop() {
	for {
		select {
		case <-w.closed:
			return
		case c := <-w.queue:
			data, err := w.roundTrip(c.msgType, c.payload)
			c.result <- callResult{data: data, err: err}
		}
	}
}

// roundTrip sends one request frame and reads one response frame.
func (w *Worker) roundTrip(msgType uint16, payload []byte) ([]byte, error) {
	// Build request.
	var buf bytes.Buffer

	hdr := reqHeader{
		Type:       msgType,
		Flags:      0,
		PayloadLen: uint32(len(payload)), //nolint:gosec
	}

	err := binary.Write(&buf, byteOrder, hdr)
	if err != nil {
		return nil, fmt.Errorf("encode header: %w", err)
	}

	buf.Write(payload)

	_, err = w.conn.Write(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("send request (type=0x%04x): %w", msgType, err)
	}

	// Read response header.
	rh, err := readRespHeader(w.conn)
	if err != nil {
		return nil, fmt.Errorf("read response header: %w", err)
	}

	if rh.Status != StatusOK {
		return nil, &ServerError{Code: rh.Status, MsgType: msgType}
	}

	// Read response body.
	if rh.ResponseLen == 0 {
		return nil, nil
	}

	body := make([]byte, rh.ResponseLen)

	_, err = io.ReadFull(w.conn, body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return body, nil
}

// ServerError is returned when the GPU server responds with a non-OK status.
type ServerError struct {
	Code    uint32
	MsgType uint16
}

func (e *ServerError) Error() string {
	var name string

	switch e.Code {
	case StatusNotImpl:
		name = "not implemented"
	case StatusBadHandle:
		name = "bad handle"
	case StatusOOM:
		name = "GPU out of memory"
	case StatusCUDA:
		name = "CUDA error"
	case StatusBadMsg:
		name = "bad message"
	default:
		name = fmt.Sprintf("code %d", e.Code)
	}

	return fmt.Sprintf("GPU server error on msg 0x%04x: %s", e.MsgType, name)
}

// IsNotImpl reports whether the error is a "not implemented" response from
// the server — useful to detect skeleton handlers during development.
func IsNotImpl(err error) bool {
	var se *ServerError
	if ok := isServerError(err, &se); ok {
		return se.Code == StatusNotImpl
	}

	return false
}

func isServerError(err error, out **ServerError) bool {
	if err == nil {
		return false
	}

	if errors.As(err, out) {
		return true
	}

	return false
}

// dialUnix retries connecting to a Unix socket until it appears (the server
// may take a moment to call listen(2) after being started).
func dialUnix(path string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)

	for {
		conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
		if err == nil {
			return conn, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for socket %s: %w", path, err)
		}

		time.Sleep(50 * time.Millisecond)
	}
}
