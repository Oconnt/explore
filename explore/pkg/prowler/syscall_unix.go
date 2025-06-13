package prowler

import (
	"golang.org/x/sys/unix"
)

func readMemory(pid int, data []byte, ptr uintptr) (int, error) {
	localIov := []unix.Iovec{
		{
			Base: &data[0],
			Len:  uint64(len(data)),
		},
	}

	remoteIov := []unix.RemoteIovec{
		{
			Base: ptr,
			Len:  len(data),
		},
	}

	return unix.ProcessVMReadv(pid, localIov, remoteIov, 0)
}

func writeMemory(pid int, data []byte, ptr uintptr) (int, error) {
	localIov := []unix.Iovec{
		{
			Base: &data[0],
			Len:  uint64(len(data)),
		},
	}

	remoteIov := []unix.RemoteIovec{
		{
			Base: ptr,
			Len:  len(data),
		},
	}

	return unix.ProcessVMWritev(pid, localIov, remoteIov, 0)
}
