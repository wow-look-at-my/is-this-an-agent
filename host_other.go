//go:build !cosmo

package agent

import "runtime"

// HostOS returns the operating system this process is running on: "linux",
// "darwin", "windows", or "" when it cannot be determined.
//
// For every build target but cosmo the binary runs on the OS it was compiled
// for, so this is runtime.GOOS and costs nothing. Only a GOOS=cosmo fat APE
// has to probe (host_cosmo.go).
func HostOS() string { return runtime.GOOS }

// HostSource names the signal HostOS decided from. Here that is the build
// target itself, which is not a probe and cannot be denied.
func HostSource() string { return sourceGOOS }
