package protocol

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

type Decoder struct {
	scanner *bufio.Scanner
}

func NewDecoder(r io.Reader) *Decoder {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	return &Decoder{scanner: scanner}
}

func (d *Decoder) Decode() (Value, error) {
	if !d.scanner.Scan() {
		return Value{}, d.scanner.Err()
	}
	line := d.scanner.Text()
	fmt.Printf("RESP:%v\n", line)
	if line == "" {
		return d.Decode()
	}
	switch line[0] {
	case '+': // Simple String
		return Value{Type: TypeSimpleString, Str: string(line[1:])}, nil
	case '-': // Error
		return Value{Type: TypeError, Str: string(line[1:])}, nil
	case ':': // Integer
		val, err := strconv.ParseInt(string(line[1:]), 10, 64)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: TypeInteger, Int: val}, nil
	case '$': // Bulk String
		return d.decodeBulkString(string(line))
	case '*': // Array
		return d.decodeArray(string(line))
	default:
		return Value{}, fmt.Errorf("decoder.go,unknown RESP type: %c,%s", line[0], line)
	}
}

func (d *Decoder) decodeBulkString(header string) (Value, error) {
	lengthStr := header[1:]
	if lengthStr == "-1" {
		return Value{Type: TypeBulkString, Null: true}, nil
	}
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return Value{}, err
	}

	if !d.scanner.Scan() {
		return Value{}, d.scanner.Err()
	}
	data := d.scanner.Text()

	// 简单实现，未处理CRLF
	if len(data) < length {
		return Value{}, fmt.Errorf("bulk string data too short")
	}
	return Value{Type: TypeBulkString, Str: data[:length]}, nil
}

func (d *Decoder) decodeArray(header string) (Value, error) {
	countStr := header[1:]
	if countStr == "-1" {
		return Value{Type: TypeArray, Null: true}, nil
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return Value{}, err
	}

	array := make([]Value, count)
	for i := 0; i < count; i++ {
		val, err := d.Decode()
		if err != nil {
			return Value{}, err
		}
		array[i] = val
	}
	return Value{Type: TypeArray, Array: array}, nil
}
