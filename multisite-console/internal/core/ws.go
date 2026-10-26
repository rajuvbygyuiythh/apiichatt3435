package core

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// A deliberately small WebSocket client.
//
// The console is stdlib-only on purpose: no module downloads, no build tags, and
// `go test ./...` runs anywhere. Chrome's DevTools protocol only needs a client,
// so a client is all this file implements (text frames, ping/pong, close).
type wsConn struct {
	conn net.Conn
	rd   *bufio.Reader

	writeMu sync.Mutex
	closeMu sync.Mutex
	closed  bool
}

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// dialWS performs the HTTP upgrade handshake and returns a frame connection.
func dialWS(rawURL string, timeout time.Duration) (*wsConn, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse websocket url: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return nil, fmt.Errorf("unsupported websocket scheme %q", parsed.Scheme)
	}

	host := parsed.Host
	if !strings.Contains(host, ":") {
		if parsed.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		conn.Close()
		return nil, fmt.Errorf("random nonce: %w", err)
	}
	key := base64.StdEncoding.EncodeToString(nonce)

	path := parsed.RequestURI()
	if path == "" {
		path = "/"
	}
	request := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + parsed.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Origin: http://" + parsed.Host + "\r\n\r\n"

	if _, err := io.WriteString(conn, request); err != nil {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake write: %w", err)
	}

	reader := bufio.NewReaderSize(conn, 64*1024)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake read: %w", err)
	}
	if !strings.Contains(statusLine, "101") {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade rejected: %s", strings.TrimSpace(statusLine))
	}

	accept := ""
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("websocket headers: %w", err)
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "sec-websocket-accept:") {
			accept = strings.TrimSpace(trimmed[len("sec-websocket-accept:"):])
		}
	}

	want := wsAccept(key)
	if accept != "" && accept != want {
		conn.Close()
		return nil, errors.New("websocket handshake: bad accept token")
	}

	_ = conn.SetDeadline(time.Time{})
	return &wsConn{conn: conn, rd: reader}, nil
}

func wsAccept(key string) string {
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// writeText sends one masked text frame (clients must mask).
func (w *wsConn) writeText(payload []byte) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()

	header := make([]byte, 0, 14)
	header = append(header, 0x81) // FIN + text

	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length)|0x80)
	case length <= 0xFFFF:
		header = append(header, 126|0x80, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(length))
	default:
		header = append(header, 127|0x80, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(length))
	}

	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return fmt.Errorf("mask key: %w", err)
	}
	header = append(header, mask...)

	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ mask[i%4]
	}

	if _, err := w.conn.Write(append(header, masked...)); err != nil {
		return fmt.Errorf("websocket write: %w", err)
	}
	return nil
}

func (w *wsConn) writeControl(opcode byte, payload []byte) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()

	header := []byte{0x80 | opcode, byte(len(payload)) | 0x80}
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	header = append(header, mask...)
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	_, err := w.conn.Write(append(header, masked...))
	return err
}

// readMessage returns the next complete text/binary message.
func (w *wsConn) readMessage() ([]byte, error) {
	var message []byte
	for {
		var header [2]byte
		if _, err := io.ReadFull(w.rd, header[:]); err != nil {
			return nil, err
		}
		fin := header[0]&0x80 != 0
		opcode := header[0] & 0x0F
		masked := header[1]&0x80 != 0
		length := int64(header[1] & 0x7F)

		switch length {
		case 126:
			var ext [2]byte
			if _, err := io.ReadFull(w.rd, ext[:]); err != nil {
				return nil, err
			}
			length = int64(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			if _, err := io.ReadFull(w.rd, ext[:]); err != nil {
				return nil, err
			}
			length = int64(binary.BigEndian.Uint64(ext[:]))
		}

		if length < 0 || length > 64<<20 {
			return nil, fmt.Errorf("websocket frame too large: %d", length)
		}

		var maskKey [4]byte
		if masked {
			if _, err := io.ReadFull(w.rd, maskKey[:]); err != nil {
				return nil, err
			}
		}

		payload := make([]byte, length)
		if length > 0 {
			if _, err := io.ReadFull(w.rd, payload); err != nil {
				return nil, err
			}
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}

		switch opcode {
		case 0x9: // ping
			_ = w.writeControl(0xA, payload)
			continue
		case 0xA: // pong
			continue
		case 0x8: // close
			w.markClosed()
			return nil, io.EOF
		case 0x0, 0x1, 0x2:
			message = append(message, payload...)
			if fin {
				return message, nil
			}
		default:
			continue
		}
	}
}

func (w *wsConn) markClosed() {
	w.closeMu.Lock()
	w.closed = true
	w.closeMu.Unlock()
}

// close sends a close frame (best effort) and closes the TCP connection.
func (w *wsConn) close() error {
	if w == nil {
		return nil
	}
	w.closeMu.Lock()
	already := w.closed
	w.closed = true
	w.closeMu.Unlock()
	if already {
		return nil
	}
	_ = w.writeControl(0x8, []byte{0x03, 0xE8})
	return w.conn.Close()
}
