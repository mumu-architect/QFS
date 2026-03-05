package protocol

import (
	"bufio"
	"fmt"
	"strconv"
)

type Encoder struct {
	writer *bufio.Writer
}

func NewEncoder(w *bufio.Writer) *Encoder {
	return &Encoder{writer: w}
}

func (e *Encoder) Encode(v Value) error {
	switch v.Type {
	case TypeSimpleString:
		_, err := e.writer.WriteString("+" + v.Str + "\r\n")
		return err
	case TypeError:
		_, err := e.writer.WriteString("-" + v.Str + "\r\n")
		return err
	case TypeInteger:
		_, err := e.writer.WriteString(":" + strconv.FormatInt(v.Int, 10) + "\r\n")
		return err
	case TypeBulkString:
		if v.Null {
			_, err := e.writer.WriteString("$-1\r\n")
			return err
		}
		_, err := e.writer.WriteString("$" + strconv.Itoa(len(v.Str)) + "\r\n" + v.Str + "\r\n")
		return err
	case TypeArray:
		if v.Null {
			_, err := e.writer.WriteString("*-1\r\n")
			return err
		}
		if err := e.writer.WriteByte('*'); err != nil {
			return err
		}
		if _, err := e.writer.WriteString(strconv.Itoa(len(v.Array)) + "\r\n"); err != nil {
			return err
		}
		for _, val := range v.Array {
			if err := e.Encode(val); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported value type: %s", v.Type)
	}
}

func (e *Encoder) Flush() error {
	return e.writer.Flush()
}
