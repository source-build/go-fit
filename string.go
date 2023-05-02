package fit

import (
	"bytes"
	"unicode/utf8"
)

// StringSpliceTag Splits the splice string with the specified symbol
func StringSpliceTag(tag string, str ...string) string {
	var buf bytes.Buffer
	for i, v := range str {
		if len(str) == i+1 {
			buf.WriteString(v)
		} else {
			buf.WriteString(v + tag)
		}
	}
	return buf.String()
}

// StringSplice Splice string
func StringSplice(str ...string) string {
	var buf bytes.Buffer
	for _, v := range str {
		buf.WriteString(v)
	}
	return buf.String()
}

// SubStrDecodeRuneInString Intercepts a string of a specified length
func SubStrDecodeRuneInString(s string, length int) string {
	var size, n int
	for i := 0; i < length && n < len(s); i++ {
		_, size = utf8.DecodeRuneInString(s[n:])
		n += size
	}
	return s[:n]
}

type JoinRowString struct {
	value string
}

func NewJoinString() *JoinRowString {
	return &JoinRowString{}
}

func (j *JoinRowString) Row(str string) *JoinRowString {
	j.value += str + "\n"
	return j
}

func (j *JoinRowString) Col(str string) *JoinRowString {
	j.value += str
	return j
}

func (j *JoinRowString) Wrap() *JoinRowString {
	j.value += "\n"
	return j
}

func (j *JoinRowString) Blank() *JoinRowString {
	j.value += "\t"
	return j
}

func (j *JoinRowString) String() string {
	return j.value
}
