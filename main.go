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
		runChild()
	default:
		runParent()
	}
}

func runParent(){ fmt.Printf("[parent] my hostname: %s\n", getHostname())
	fmt.Println("[parent] spawning child in new UTS namespace...")

	// configure Cmd struct
	cmd := exec.Command("/proc/self/exe")

	//set I/O file descriptors for child process
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "ROLE=child")

	// set clone flag for child's process namespace (UTS, PID< and Mount)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[parent] child exited with error: %v\n", err)
		os.Exit(1)
	}

	// Prove that the host's hostname is untouched.
	fmt.Printf("[parent] hostname after child exits: %s\n", getHostname())
	fmt.Printf("[parent]  PID: %d\n", os.Getpid())
}

func runChild() {

    fmt.Printf("[child]  pid inside namespace: %d\n", os.Getpid())
    fmt.Printf("[child]  hostname before change: %s\n", getHostname())

    if err := syscall.Sethostname([]byte("my-container")); err != nil {
        fmt.Fprintf(os.Stderr, "[child] sethostname failed: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("[child]  hostname after change:  %s\n", getHostname())

    // Break mount propagation BEFORE mounting anything,
    // so changes stay local to this mount namespace.
    if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
        fmt.Fprintf(os.Stderr, "[child] mount private failed: %v\n", err)
        os.Exit(1)
    }

    // Mount procfs instance bound to THIS PID namespace.
    if err := syscall.Mount("proc", "/proc", "proc",
        uintptr(syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV), ""); err != nil {
        fmt.Fprintf(os.Stderr, "[child] mount /proc failed: %v\n", err)
        os.Exit(1)
    }

    defer syscall.Unmount("/proc", 0)

    /*
    TODO: bind mount rootfs onto itself

    if err := syscall.Mount("rootfs", "rootfs", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
        fmt.Fprintf(os.Stderr, "[child] mount private failed: %v\n", err)
        os.Exit(1)
    }
    */

    /*
    TODO: chdir into rootfs & call pivot_rootf
    */

    fmt.Println("[child]  running `ps aux`:")
    psCmd := exec.Command("ps", "aux")
    psCmd.Stdout = os.Stdout
    psCmd.Stderr = os.Stderr
    psCmd.Run()
}


func getHostname() string{
	h, err := os.Hostname()

	if err != nil{
		return "<error>"
	}
	return h
}
