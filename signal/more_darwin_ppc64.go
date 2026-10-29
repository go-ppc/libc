// Copyright 2026 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Port of more_darwin_amd64.go; Leopard's values are the same (measured in
// the captured headers, runs/a1-capture-20261029): sys/signal.h:130-137
// defines SIG_DFL and SIG_IGN as handler casts to 0 and 1.

package signal

const (
	//	sys/signal.h:130:#define	SIG_DFL		(void (*)(int))0	/* default signal handling */
	SIG_DFL = 0
	//	sys/signal.h:131:#define	SIG_IGN		(void (*)(int))1	/* ignore signal */
	SIG_IGN = 1
)
