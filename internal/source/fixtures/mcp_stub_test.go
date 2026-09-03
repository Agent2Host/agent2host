package fixtures_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent2host/agent2host/internal/source/fixtures"
)

func TestClubDatabaseMCPStubHandshake(t *testing.T) {
	root, err := fixtures.Root()
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "trees", "valid", "club-system", "mcp", "club-database.py")

	initReq := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	listReq := map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}}
	callReq := map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{
			"name": "search_policy", "arguments": map[string]any{"query": "guest"},
		},
	}

	initResp, err := mcpRoundTrip(t, script, initReq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(initResp), "club-database") {
		t.Fatalf("initialize: %s", initResp)
	}

	listResp, err := mcpRoundTrip(t, script, initReq, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{}}, listReq)
	if err != nil {
		t.Fatal(err)
	}
	var listDoc struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listResp, &listDoc); err != nil {
		t.Fatalf("tools/list parse: %v body=%s", err, listResp)
	}
	names := map[string]bool{}
	for _, tool := range listDoc.Result.Tools {
		names[tool.Name] = true
	}
	if !names["search_policy"] || !names["get_member"] || names["forbidden_echo"] {
		t.Fatalf("tools/list names: %v", names)
	}

	callResp, err := mcpRoundTrip(t, script, initReq, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{}}, callReq)
	if err != nil {
		t.Fatal(err)
	}
	body := string(callResp)
	if !strings.Contains(body, "POLICY_CANARY_OK") || !strings.Contains(body, "members only after 18:00") {
		t.Fatalf("tools/call: %s", body)
	}

	// Hosts may speak NDJSON instead of Content-Length; both must work.
	ndjsonOut, err := mcpRoundTripNDJSON(t, script, initReq, listReq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ndjsonOut), "search_policy") {
		t.Fatalf("ndjson tools/list: %s", ndjsonOut)
	}
}

func mcpRoundTripNDJSON(t *testing.T, script string, msgs ...map[string]any) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("python3", "-u", script)
	var stdin bytes.Buffer
	for _, msg := range msgs {
		raw, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		stdin.Write(raw)
		stdin.WriteByte('\n')
	}
	cmd.Stdin = &stdin
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, err
		}
	}
	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	if len(lines) == 0 {
		return nil, io.EOF
	}
	return lines[len(lines)-1], nil
}

func mcpRoundTrip(t *testing.T, script string, msgs ...map[string]any) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("python3", script)
	var stdin bytes.Buffer
	for _, msg := range msgs {
		raw, err := json.Marshal(msg)
		if err != nil {
			return nil, err
		}
		stdin.WriteString("Content-Length: ")
		stdin.WriteString(itoa(len(raw)))
		stdin.WriteString("\r\n\r\n")
		stdin.Write(raw)
	}
	cmd.Stdin = &stdin
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, err
		}
	}
	// Return last JSON-RPC response frame body.
	return lastMCPJSON(out)
}

func lastMCPJSON(out []byte) ([]byte, error) {
	r := bufio.NewReader(bytes.NewReader(out))
	var last []byte
	for {
		headers := map[string]string{}
		for {
			line, err := readLine(r)
			if err != nil {
				if err == io.EOF && len(last) > 0 {
					return last, nil
				}
				return last, err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			k, v, _ := strings.Cut(line, ":")
			headers[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
		n := atoi(headers["content-length"])
		if n <= 0 {
			continue
		}
		body := make([]byte, n)
		if _, err := io.ReadFull(r, body); err != nil {
			return last, err
		}
		last = body
	}
}

func readLine(r *bufio.Reader) (string, error) {
	return r.ReadString('\n')
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
