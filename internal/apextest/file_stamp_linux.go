//go:build linux

package apextest

import "syscall"

func statMtimeNanos(st *syscall.Stat_t) int64 { return st.Mtim.Nano() }
func statCtimeNanos(st *syscall.Stat_t) int64 { return st.Ctim.Nano() }
