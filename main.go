package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	// check roles
	switch os.Getenv("ROLE"){
	case "child":
		//do something
		runChild()
	default:
		runParent()
		//call parent
	}
}

func runParent(){
	fmt.Printf("[parent] my hostname: %s\n", getHostname())
	fmt.Println("[parent] spawning child in new UTS namespace...")

	// configure Cmd struct
	cmd := exec.Command("/proc/self/exe")

	//set I/O file descriptors for child process
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "ROLE=child")

	// set clone flag for child's process new namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS,
	}

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[parent] child exited with error: %v\n", err)
		os.Exit(1)
	}

	// Prove that the host's hostname is untouched.
	fmt.Printf("[parent] hostname after child exits: %s\n", getHostname())
}

func runChild() {
    fmt.Printf("[child]  hostname before change: %s\n", getHostname())

    // syscall.Sethostname changes the hostname in THIS process's
    // UTS namespace only. The parent's namespace is untouched.
    if err := syscall.Sethostname([]byte("my-container")); err != nil {
        fmt.Fprintf(os.Stderr, "[child] sethostname failed: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("[child]  hostname after change:  %s\n", getHostname())
}

func getHostname() string{
	h, err := os.Hostname()

	if err != nil{
		return "<error>"
	}
	return h
}
