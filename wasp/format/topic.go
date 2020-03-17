package format

import "bytes"

var (
	SEP = byte('/')
)

type Topic []byte

func (t Topic) Next() (Topic, string) {
	end := bytes.IndexByte(t, SEP)
	if end < 0 {
		return nil, string(t)
	}
	return t[end+1:], string(t[:end])
}
