package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type UnshareNetnsRequest struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
}

type UnshareNetnsResponse struct {
	ParentPID int `json:"parent_pid"`
	ChildPID  int `json:"child_pid"`
}

func main() {
	serverURL := "http://127.0.0.1:8080/unshare/netns"

	req := UnshareNetnsRequest{
		Path: "/bin/sh",
		Args: []string{"-c", "sleep 30"},
	}

	resp, err := callUnshareNetns(serverURL, req)
	if err != nil {
		fail("API call failed: %v", err)
	}

	fmt.Printf("parent_pid=%d child_pid=%d\n", resp.ParentPID, resp.ChildPID)

	// child가 exec까지 완료할 시간을 약간 준다.
	time.Sleep(300 * time.Millisecond)

	parentNetns, err := readNetns(resp.ParentPID)
	if err != nil {
		fail("failed to read parent netns: %v", err)
	}

	childNetns, err := readNetns(resp.ChildPID)
	if err != nil {
		fail("failed to read child netns: %v", err)
	}

	fmt.Printf("parent netns: %s\n", parentNetns)
	fmt.Printf("child  netns: %s\n", childNetns)

	if parentNetns == childNetns {
		fail("netns check failed: parent and child are in the same network namespace")
	}

	cmdline, err := readCmdline(resp.ChildPID)
	if err != nil {
		fail("failed to read child cmdline: %v", err)
	}

	fmt.Printf("child cmdline: %q\n", cmdline)

	if !strings.Contains(cmdline, "/bin/sh") && !strings.Contains(cmdline, "sh") {
		fail("cmdline check failed: child does not look like /bin/sh")
	}

	pass("all checks passed")
}

func callUnshareNetns(url string, req UnshareNetnsRequest) (*UnshareNetnsResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpResp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, _ := io.ReadAll(httpResp.Body)

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status=%d body=%s", httpResp.StatusCode, string(respBody))
	}

	var resp UnshareNetnsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	if resp.ParentPID <= 0 {
		return nil, fmt.Errorf("invalid parent_pid: %d", resp.ParentPID)
	}

	if resp.ChildPID <= 0 {
		return nil, fmt.Errorf("invalid child_pid: %d", resp.ChildPID)
	}

	if resp.ParentPID == resp.ChildPID {
		return nil, fmt.Errorf("parent_pid and child_pid must be different")
	}

	return &resp, nil
}

func readNetns(pid int) (string, error) {
	return os.Readlink(fmt.Sprintf("/proc/%d/ns/net", pid))
}

func readCmdline(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", err
	}

	// /proc/<pid>/cmdline은 NUL byte로 인자가 구분된다.
	parts := strings.Split(string(data), "\x00")

	var cleaned []string
	for _, p := range parts {
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}

	return strings.Join(cleaned, " "), nil
}

func pass(format string, args ...any) {
	fmt.Printf("[PASS] "+format+"\n", args...)
	os.Exit(0)
}

func fail(format string, args ...any) {
	fmt.Printf("[FAIL] "+format+"\n", args...)
	os.Exit(1)
}
