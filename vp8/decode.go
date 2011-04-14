// Copyright 2011 The vp8-go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vp8 implements a vp8 image and video decoder.
//
// The VP8 specification is at
// http://webmproject.org/media/pdf/vp8-bitstream.pdf.
package vp8

import (
	"image"
	"image/ycbcr"
	"io"
	"os"
)

// limitReader wraps an io.Reader to read at most n bytes from it.
type limitReader struct {
	r io.Reader
	n int
}

// ReadFull reads exactly len(p) bytes into p.
func (r *limitReader) ReadFull(p []byte) os.Error {
	if len(p) > r.n {
		return io.ErrUnexpectedEOF
	}
	n, err := io.ReadFull(r.r, p)
	r.n -= n
	return err
}

// FrameHeader is a frame header, specified in section 9.1.
type FrameHeader struct {
	KeyFrame          bool
	VersionNumber     uint8
	ShowFrame         bool
	FirstPartitionLen uint32
	Width             int
	Height            int
	XScale            uint8
	YScale            uint8
}

// Decoder decodes VP8 bitstreams into frames. Decoding one frame consists of
// calling Init, DecodeFrameHeader and then DecodeFrame in that order.
// A Decoder can be re-used to decode multiple frames.
type Decoder struct {
	// r is the input bitsream.
	r limitReader
	// img is the YCbCr image to decode into.
	img *ycbcr.YCbCr
	// mbw and mbh are the number of 16x16 macroblocks wide and high the image is.
	mbw, mbh int
	// frameHeader is the frame header. When decoding multiple frames,
	// frames that aren't key frames will inherit the Width, Height,
	// XScale and YScale of the most recent key frame.
	frameHeader FrameHeader
	// The image data is divided into a number of independent partitions.
	// There is 1 "first partition" and between 1 and 8 "other partitions"
	// for coefficient data, specified in section 9.5.
	fp partition
	op [8]partition
	// scratch is a scratch buffer.
	scratch [10]byte
}

// NewDecoder returns a new Decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Init initializes the decoder to read at most n bytes from r.
func (d *Decoder) Init(r io.Reader, n int) {
	d.r = limitReader{r, n}
}

// DecodeFrameHeader decodes the frame header.
func (d *Decoder) DecodeFrameHeader() (fh FrameHeader, err os.Error) {
	// All frame headers are at least 3 bytes long.
	b := d.scratch[:3]
	if err = d.r.ReadFull(b); err != nil {
		return
	}
	d.frameHeader.KeyFrame = (b[0] & 1) == 0
	d.frameHeader.VersionNumber = (b[0] >> 1) & 7
	d.frameHeader.ShowFrame = (b[0]>>4)&1 == 1
	d.frameHeader.FirstPartitionLen = uint32(b[0])>>5 | uint32(b[1])<<3 | uint32(b[2])<<11
	if !d.frameHeader.KeyFrame {
		return d.frameHeader, nil
	}
	// Frame headers for key frames are an additional 7 bytes long.
	b = d.scratch[:7]
	if err = d.r.ReadFull(b); err != nil {
		return
	}
	// Check the magic sync code.
	if b[0] != 0x9d || b[1] != 0x01 || b[2] != 0x2a {
		err = os.NewError("vp8: invalid format")
		return
	}
	d.frameHeader.Width = int(b[4]&0x3f)<<8 | int(b[3])
	d.frameHeader.Height = int(b[6]&0x3f)<<8 | int(b[5])
	d.frameHeader.XScale = b[4] >> 6
	d.frameHeader.YScale = b[6] >> 6
	d.mbw = (d.frameHeader.Width + 0x0f) >> 4
	d.mbh = (d.frameHeader.Height + 0x0f) >> 4
	return d.frameHeader, nil
}

// ensureImg ensures that d.img is large enough to hold the decoded frame.
func (d *Decoder) ensureImg() {
	if d.img != nil {
		p0, p1 := d.img.Rect.Min, d.img.Rect.Max
		if p0.X == 0 && p0.Y == 0 && p1.X >= 16*d.mbw && p1.Y >= 16*d.mbh {
			return
		}
	}
	n := d.mbw * d.mbh
	// VP8 always uses 4:2:0 chroma subsampling, so each macroblock consists of
	// 1x16x16 luma samples and 2x8x8 chroma samples.
	buf := make([]uint8, n*(1*16*16+2*8*8))
	d.img = &ycbcr.YCbCr{
		Y:              buf[n*(0*16*16+0*8*8) : n*(1*16*16+0*8*8)],
		Cb:             buf[n*(1*16*16+0*8*8) : n*(1*16*16+1*8*8)],
		Cr:             buf[n*(1*16*16+1*8*8) : n*(1*16*16+2*8*8)],
		SubsampleRatio: ycbcr.SubsampleRatio420,
		YStride:        d.mbw * 16,
		CStride:        d.mbw * 8,
		Rect:           image.Rect(0, 0, d.frameHeader.Width, d.frameHeader.Height),
	}
}

// parseOtherHeaders parses header information other than the frame header.
func (d *Decoder) parseOtherHeaders() os.Error {
	// Initialize and parse the first partition.
	firstPartition := make([]byte, d.frameHeader.FirstPartitionLen)
	if err := d.r.ReadFull(firstPartition); err != nil {
		return err
	}
	d.fp.init(firstPartition)
	if d.frameHeader.KeyFrame {
		// Read and ignore the color space and pixel clamp values. They are
		// specified in section 9.2, but we do not implement those features.
		d.fp.readUint(uniformProb, 1)
		d.fp.readUint(uniformProb, 1)
	}
	// TODO(nigeltao): parse all the other header fields.
	// TODO(nigeltao): initialize the other partitions.
	if d.fp.unexpectedEOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// DecodeFrame decodes the frame and returns it as an YCbCr image.
// The image's contents are valid up until the next call to Decoder.Init.
func (d *Decoder) DecodeFrame() (*ycbcr.YCbCr, os.Error) {
	d.ensureImg()
	if err := d.parseOtherHeaders(); err != nil {
		return nil, err
	}
	// TODO(nigeltao): actually decode the image.
	return d.img, nil
}
