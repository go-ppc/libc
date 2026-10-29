// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Port of more_linux_386.go. Values from Adelie's musl <signal.h>
// (runs/libc-linuxppc-20261029/capture/extract-manifest.sha256 in the port
// workspace), which defines them as handler casts ccgo cannot export.

package signal

const (
	//	signal.h:284:#define SIG_DFL  ((void (*)(int)) 0)
	SIG_DFL = 0
	//	signal.h:285:#define SIG_IGN  ((void (*)(int)) 1)
	SIG_IGN = 1
)
