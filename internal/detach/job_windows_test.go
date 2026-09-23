package detach

import (
	"os"
	"slices"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type pidList struct {
	assigned uint32
	listed   uint32
	pids     [64]uintptr
}

func newJob(t *testing.T, limits uint32) windows.Handle {
	t.Helper()
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = limits
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		t.Fatal(err)
	}
	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		t.Fatal(err)
	}
	return job
}

func inJob(t *testing.T, job windows.Handle, pid int) bool {
	t.Helper()
	var list pidList
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&list)), uint32(unsafe.Sizeof(list)), nil); err != nil {
		t.Fatal(err)
	}
	return slices.Contains(list.pids[:list.listed], uintptr(pid))
}

const sleeperEnv = "CREDS_DETACH_SLEEPER"

func TestSleeper(t *testing.T) {
	if os.Getenv(sleeperEnv) == "" {
		t.Skip("helper process for spawnSleeper")
	}
	time.Sleep(3 * time.Second)
}

func spawnSleeper(t *testing.T) int {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := Command(exe, "-test.run=^TestSleeper$")
	cmd.Env = append(os.Environ(), sleeperEnv+"=1")
	p, err := start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Release() })
	return p.Pid
}

func TestChildLeavesTheJob(t *testing.T) {
	open := newJob(t, windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK)
	if inJob(t, open, spawnSleeper(t)) {
		t.Fatal("child stayed in a job that allows breakaway")
	}
	closed := newJob(t, 0)
	if !inJob(t, closed, spawnSleeper(t)) {
		t.Fatal("child not started inside a job that refuses breakaway")
	}
}
