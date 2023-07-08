package parser

import (
	"bufio"
	"io"
)

// splitFunc is a split function for a bufio.Scanner that splits a sequence of
// bytes into SSE events. Each event ends with two consecutive newline sequences,
// where a newline sequence is defined as either "\n", "\r", or "\r\n".
var splitFunc bufio.SplitFunc = func(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		return 0, nil, nil
	}

	var start, index, endlineLen int
	for {
		index, endlineLen = NewlineIndex(data[advance:])
		advance += index + endlineLen
		if index == 0 {
			start += endlineLen
		}
		if advance == len(data) || (isNewlineChar(data[advance]) && index > 0) {
			break
		}
	}

	if l := len(data); advance == l && !atEOF {
		// We have reached the end of the buffer but have not yet seen two consecutive
		// newline sequences, so we request more data.
		return 0, nil, nil
	} else if advance < l {
		// We have found a newline. Consume the end-of-line sequence.
		advance++
		// Consume one more character if end-of-line is \r\n.
		if advance < l && data[advance-1] == '\r' && data[advance] == '\n' {
			advance++
		}
	}

	token = data[start:advance]

	return advance, token, nil
}

// ReaderParser extracts fields from a reader. Reading is buffered using a bufio.Scanner.
// The ReaderParser also removes the UTF-8 BOM if it exists.
type ReaderParser struct {
	sc *bufio.Scanner
	p  *ByteParser
}

// Scan parses a single field from the reader. It returns false when there are no more fields to parse.
func (r *ReaderParser) Scan() bool {
	if !r.p.Scan() {
		if !r.sc.Scan() {
			return false
		}

		r.p.Reset(r.sc.Bytes())

		return r.p.Scan()
	}

	return true
}

// Field returns the last parsed field. The Field's Value byte slice isn't owned by the field, the underlying buffer
// is owned by the bufio.Scanner that is used by the ReaderParser.
func (r *ReaderParser) Field() Field {
	return r.p.Field()
}

// Err returns the last read error.
func (r *ReaderParser) Err() error {
	if r.sc.Err() != nil {
		return r.sc.Err()
	}
	return r.p.Err()
}

// NewReaderParser returns a parser that extracts fields from a reader.
func NewReaderParser(r io.Reader) *ReaderParser {
	sc := bufio.NewScanner(RemovePrefix(r, "\xEF\xBB\xBF"))
	sc.Split(splitFunc)

	return &ReaderParser{sc: sc, p: NewByteParser(nil)}
}
