//go:build windows

package sandbox

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// windowsJob is a job object configured with KILL_ON_JOB_CLOSE: closing the
// handle terminates every process in the job, including descendants of the
// sandboxed command. This gives reliable whole-tree kills on Windows, where
// killing the direct child leaves grandchildren running.
type windowsJob struct {
	handle windows.Handle
}

// newWindowsJob creates a job object with KILL_ON_JOB_CLOSE.
func newWindowsJob() (*windowsJob, error) {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	j := &windowsJob{handle: h}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(h,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		j.close()
		return nil, err
	}
	return j, nil
}

// assignByPID puts the process with the given pid (and every process it
// spawns) into the job. The process is opened by PID, avoiding the
// unexported os.Process internals.
func (j *windowsJob) assignByPID(pid int) error {
	proc, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(pid),
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(proc)
	return windows.AssignProcessToJobObject(j.handle, proc)
}

// close closes the handle; KILL_ON_JOB_CLOSE then terminates the job. It is
// idempotent.
func (j *windowsJob) close() {
	if j.handle != 0 {
		_ = windows.CloseHandle(j.handle)
		j.handle = 0
	}
}
