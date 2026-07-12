package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

var rootfs = "/home/entuyenabo/projects/rockbox/rootfs"

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

    // Break mount propagation before doing anything else.
    if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
        fmt.Fprintf(os.Stderr, "[child] mount private failed: %v\n", err)
        os.Exit(1)
    }

    // Bind-mount rootfs onto itself to make it a valid mount point
    // for pivot_root — MS_BIND creates the bind mount, MS_REC applies
    // it recursively so any mounts inside rootfs are also bound.
    if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
        fmt.Fprintf(os.Stderr, "[child] bind mount rootfs failed: %v\n", err)
        os.Exit(1)
    }

    // Create the directory inside rootfs where the old root will land.
    if err := os.MkdirAll(rootfs+"/.old_root", 0700); err != nil {
        fmt.Fprintf(os.Stderr, "[child] mkdir .old_root failed: %v\n", err)
        os.Exit(1)
    }

    // Step into the new root before calling pivot_root.
    if err := syscall.Chdir(rootfs); err != nil {
        fmt.Fprintf(os.Stderr, "[child] chdir into rootfs failed: %v\n", err)
        os.Exit(1)
    }

    // Swap the root — current dir (.) becomes new /, old root lands at .old_root.
    if err := syscall.PivotRoot(".", ".old_root"); err != nil {
        fmt.Fprintf(os.Stderr, "[child] pivot_root failed: %v\n", err)
        os.Exit(1)
    }

    // Reset cwd to / inside the new root — without this the cwd
    // is a dangling reference into the old filesystem.
    if err := syscall.Chdir("/"); err != nil {
        fmt.Fprintf(os.Stderr, "[child] chdir / failed: %v\n", err)
        os.Exit(1)
    }

    // Detach the old root — MNT_DETACH means "unmount even if busy,
    // once all existing references are closed."
    if err := syscall.Unmount("/.old_root", syscall.MNT_DETACH); err != nil {
        fmt.Fprintf(os.Stderr, "[child] unmount old root failed: %v\n", err)
        os.Exit(1)
    }

    if err := os.Remove("/.old_root"); err != nil {
        fmt.Fprintf(os.Stderr, "[child] remove .old_root failed: %v\n", err)
        os.Exit(1)
    }

    // NOW mount procfs — we're inside the new root, so this lands
    // on Alpine's /proc, bound to our PID namespace.
    if err := syscall.Mount("proc", "/proc", "proc",
        uintptr(syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV), ""); err != nil {
        fmt.Fprintf(os.Stderr, "[child] mount /proc failed: %v\n", err)
        os.Exit(1)
    }

    // Replace this process with a shell — execve, not fork+exec.
    // argv[0] is the program name by convention.
    if err := syscall.Exec("/bin/sh", []string{"sh"}, os.Environ()); err != nil {
        fmt.Fprintf(os.Stderr, "[child] exec /bin/sh failed: %v\n", err)
        os.Exit(1)
    }
}


func getHostname() string{
	h, err := os.Hostname()

	if err != nil{
		return "<error>"
	}
	return h
}
