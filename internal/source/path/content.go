package path

import (
	"bytes"
	"math"
	"regexp"
	"unicode/utf8"
)

var pemHeaders = [][]byte{
	[]byte("-----BEGIN PRIVATE KEY-----"),
	[]byte("-----BEGIN ENCRYPTED PRIVATE KEY-----"),
	[]byte("-----BEGIN RSA PRIVATE KEY-----"),
	[]byte("-----BEGIN EC PRIVATE KEY-----"),
	[]byte("-----BEGIN DSA PRIVATE KEY-----"),
	[]byte("-----BEGIN OPENSSH PRIVATE KEY-----"),
}

// HasPEM reports SRC-CONTENT-PRIVATE-KEY headers (header plus newline or end).
// Every occurrence of each closed header is checked; a decoy match must not
// hide a later real header.
func HasPEM(raw []byte) bool {
	for _, h := range pemHeaders {
		off := 0
		for {
			i := bytes.Index(raw[off:], h)
			if i < 0 {
				break
			}
			i += off
			rest := raw[i+len(h):]
			if len(rest) == 0 || rest[0] == '\n' || rest[0] == '\r' {
				return true
			}
			off = i + 1
		}
	}
	return false
}

var awsKey = regexp.MustCompile(`AKIA[A-Z0-9]{16}`)

// ScanWarn reports SRC-CONTENT-SCAN-WARN heuristics. Never fails register.
func ScanWarn(raw []byte) bool {
	if utf8.Valid(raw) && awsKey.Find(raw) != nil {
		return true
	}
	return highEntropy(raw)
}

func highEntropy(raw []byte) bool {
	if len(raw) < 32 {
		return false
	}
	var freq [256]int
	for _, b := range raw {
		freq[b]++
	}
	n := float64(len(raw))
	var h float64
	for _, c := range freq {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h >= 7.5
}
