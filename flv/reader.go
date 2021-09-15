package flv

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
)

// Reader reads FLV header and tags from an input stream.
type Reader struct {
	*fileReader
}

// NewReader returns a new reader that reads from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{newFileReader(r)}
}

// ReadHeader reads FLV header
/*
FLV文件头由9bytes组成，前3个bytes是文件类型，总是“FLV”，也就是（0x46 0x4C 0x56）。第4btye是版本号，目前一般是0x01。
第5byte是流的信息，倒数第一bit是1表示有视频（0x01），倒数第三bit是1表示有音频（0x4），有视频又有音频就是0x01 | 0x04（0x05），其他都应该是0。
最后4bytes表示FLV 头的长度，3+1+1+4 = 9。
*/
func (r *Reader) ReadHeader() (*Header, error) {
	b, err := r.next(9)
	if err != nil {
		return nil, err
	}
	if getUint24(b[0:]) != signature {
		return nil, fmt.Errorf("flv: incorrect signature: 0x%x", hex.EncodeToString(b[0:3]))
	}
	if b[3] != 1 {
		return nil, fmt.Errorf("flv: unsupported version: %d", b[3])
	}
	r.skip(int(getUint32(b[5:])) - 9)
	return &Header{b[4]}, nil
}

// ReadTag reads FLV tag and returns payload reader.
// Reader is not valid after next ReadTag.
/*
FLV body由若干个tag 组成。每一个tag第一部分是tag header，tag header长度为11bytes，但是每个tag header前面有4bytes记录着上一个tag的长度。
tag header：

        １）第1个byte为记录着tag的类型，音频（0x8），视频（0x9），脚本（0x12）；

        ２）第2到4bytes是数据区的长度，也就是tag data的长度；

        ３）再后面3个bytes是时间戳，单位是毫秒，类型为0x12则时间戳为0，时间戳控制着文件播放的速度，可以根据音视频的帧率类设置；

        ４）时间戳后面一个byte是扩展时间戳，时间戳不够长的时候用；

        ５）最后3bytes是streamID，但是总为0，再后面就是数据区了（tag data），也即是h264的裸流；

        ６）tag header 长度为1+3+3+1+3=11。
*/
func (r *Reader) ReadTag() (*Tag, io.Reader, error) {
	b, err := r.next(15)
	if err != nil {
		return nil, nil, err
	}
	tag := &Tag{
		Type:   b[4],
		Size:   getInt24(b[5:]),
		Time:   getTime(b[8:]),
		Stream: getUint24(b[12:]),
	}
	data, err := r.reader(tag.Size)
	if err != nil {
		return nil, nil, err
	}
	return tag, data, nil
}

type fileReader struct {
	r io.Reader
	b *bufio.Reader
	s io.ReadSeeker
	l *io.LimitedReader
}

func newFileReader(r io.Reader) *fileReader {
	b, ok := r.(*bufio.Reader)
	if !ok {
		b = bufio.NewReader(r)
	}
	s, _ := r.(io.ReadSeeker)
	return &fileReader{r, b, s, &io.LimitedReader{R: b, N: 0}}
}

func (r *fileReader) validate() error {
	if r.l.N <= 0 {
		return nil
	}
	b, n := int64(r.b.Buffered()), r.l.N
	r.l.N = 0
	if b < n && r.s != nil {
		r.b.Reset(r.r)
		_, err := r.s.Seek(n-b, io.SeekCurrent)
		return err
	}
	_, err := r.b.Discard(int(n))

	return err
}

func (r *fileReader) next(n int) ([]byte, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	buf, err := r.b.Peek(n)
	if err != nil {
		return nil, err
	}
	r.l.N = int64(n)
	return buf, err
}

func (r *fileReader) skip(n int) {
	if n > 0 {
		r.l.N += int64(n)
	}
}

func (r *fileReader) reader(n int) (io.Reader, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	r.l.N = int64(n)
	return r.l, nil
}

func getInt24(b []byte) int {
	_ = b[2]
	return int(b[2]) | int(b[1])<<8 | int(b[0])<<16
}

func getUint24(b []byte) uint32 {
	_ = b[2]
	return uint32(b[2]) | uint32(b[1])<<8 | uint32(b[0])<<16
}

func putUint24(b []byte, v uint32) {
	_ = b[2]
	b[2], b[1], b[0] = uint8(v), uint8(v>>8), uint8(v>>16)
}

func getTime(b []byte) int64 {
	_ = b[3]
	return int64(b[2]) | int64(b[1])<<8 | int64(b[0])<<16 | int64(b[3])<<24
}

func putTime(b []byte, v int64) {
	_ = b[3]
	b[2], b[1], b[0], b[3] = uint8(v), uint8(v>>8), uint8(v>>16), uint8(v>>24)
}

func getUint32(b []byte) uint32 {
	_ = b[3]
	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
}

func putUint32(b []byte, v uint32) {
	_ = b[3]
	b[3], b[2], b[1], b[0] = uint8(v), uint8(v>>8), uint8(v>>16), uint8(v>>24)
}
