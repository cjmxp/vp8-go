# Copyright 2011 The vp8-go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.inc

all:	install

install:
	cd vp8 && gomake install
	cd webp && gomake install

clean:
	cd vp8 && gomake clean
	cd webp && gomake clean

nuke:
	cd vp8 && gomake nuke
	cd webp && gomake nuke
