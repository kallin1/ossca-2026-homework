package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"syscall"
)

type RunRequest struct {
	Path string   `json:"path"`
	Args []string `json:"args"`
}

type RunResponse struct {
	ParentPID int `json:"parent_pid"`
	ChildPID  int `json:"child_pid"`
}

func handleUnshareNetns(w http.ResponseWriter, r *http.Request) {
	// POST만 허용
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// JSON 파싱
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// path는 절대 경로
	if len(req.Path) == 0 || req.Path[0] != '/' {
		http.Error(w, "path must be an absolute path", http.StatusBadRequest)
		return
	}

	// 자식프로세스 생성
	cmd := exec.Command(req.Path, req.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Unshareflags: syscall.CLONE_NEWNET,
	}

	// 자식 프로세스 시작
	if err := cmd.Start(); err != nil {
		http.Error(w, "failed to start child process", http.StatusInternalServerError)
		return
	}

	// parent, child pid
	childPID := cmd.Process.Pid
	parentPID := os.Getpid()

	// zombie 방지 — 백그라운드에서 wait
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "child %d exited with: %v\n", childPID, err)
		} else {
			fmt.Fprintf(os.Stderr, "child %d exited cleanly\n", childPID)
		}
	}()

	//응답
	res := RunResponse{
		ParentPID: parentPID,
		ChildPID:  childPID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func main() {
	http.HandleFunc("/unshare/netns", handleUnshareNetns)

	addr := ":8080"
	fmt.Printf("server listening on %s (pid=%d)\n", addr, os.Getpid())
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}
