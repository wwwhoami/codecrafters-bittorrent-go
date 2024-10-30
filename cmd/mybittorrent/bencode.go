package main

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// decodeStr reads a string from the reader with the length
// specified by the integer that precedes the string with a colon.
// Example: 4:spam -> spam
func decodeStr(r *bufio.Reader) (string, error) {
	bytes, err := decodeByteString(r)

	return string(bytes), err
}

func decodeByteString(r *bufio.Reader) ([]byte, error) {
	length, err := parseInt(r, ':')
	if err != nil {
		return nil, fmt.Errorf("invalid string length format: %v", err)
	} else if length < 0 {
		return nil, fmt.Errorf("invalid string length: %d", length)
	}

	if peekBuf, peekErr := r.Peek(length); peekErr != nil {
		_, err := r.Discard(length)
		return peekBuf, err
	}

	buf := make([]byte, length)

	_, err = r.Read(buf)

	return buf, err
}

// parseInt reads a number from the reader until the delimiter character is found.
// The delimiter character is not included in the returned string.
func parseInt(r *bufio.Reader, delim byte) (int, error) {
	str, err := r.ReadString(delim)
	if err != nil {
		return 0, err
	}
	// Discard delim character
	str = str[:len(str)-1]
	return strconv.Atoi(str)
}

// decodeInt reads a number from the reader until the 'e' delimiter character is found.
// The delimiter character is not included in the returned string.
// Example: i42e -> 42
func decodeInt(r *bufio.Reader) (int, error) {
	if ch, _, err := r.ReadRune(); ch != 'i' {
		return 0, fmt.Errorf("integer should start with 'i' character")
	} else if err != nil {
		return 0, err
	}

	val, err := parseInt(r, 'e')
	if err != nil {
		return 0, fmt.Errorf("invalid becoded integer: %v", err)
	}

	return val, nil
}

// decodeList reads a list from the reader until the 'e' character is found.
// The 'e' character is not included in the returned list.
// Example: l4:spam4:eggse -> ["spam", "eggs"]
func decodeList(r *bufio.Reader) (list []any, err error) {
	if ch, _, err := r.ReadRune(); ch != 'l' {
		return nil, fmt.Errorf("list should start with 'l' character")
	} else if err != nil {
		return nil, err
	}

	list = make([]any, 0)

	for {
		ch, err := r.Peek(1)
		if err != nil {
			return nil, err
		}

		if ch[0] == 'e' {
			// Discard 'e', proceeding reader by one byte
			if _, err := r.Discard(1); err != nil {
				return nil, err
			}

			break
		}

		value, err := decodeValue(r)
		if err != nil {
			return nil, err
		}

		list = append(list, value)
	}

	return
}

func decodeDict(r *bufio.Reader) (dict map[string]any, err error) {
	if ch, _, err := r.ReadRune(); ch != 'd' {
		return nil, fmt.Errorf("dictionary should start with 'd' character")
	} else if err != nil {
		return nil, err
	}

	dict = make(map[string]any)

	for {
		ch, err := r.Peek(1)
		if err != nil {
			return nil, err
		}

		if ch[0] == 'e' {
			// Discard 'e', proceeding reader by one byte
			if _, err := r.Discard(1); err != nil {
				return nil, err
			}

			break
		}

		key, err := decodeStr(r)
		if err != nil {
			return nil, err
		}

		value, err := decodeValue(r)
		if err != nil {
			return nil, err
		}

		dict[key] = value
	}

	return
}

// decodeValue decodes a bencoded value from the reader.
func decodeValue(r *bufio.Reader) (any, error) {
	ch, err := r.Peek(1)
	if err != nil {
		return nil, err
	}
	firstCh := ch[0]

	switch {
	case unicode.IsDigit(rune(firstCh)) || firstCh == '-':
		return decodeStr(r)
	case firstCh == 'i':
		return decodeInt(r)
	case firstCh == 'l':
		return decodeList(r)
	case firstCh == 'd':
		return decodeDict(r)
	default:
		return "", fmt.Errorf("invalid data type: %c", firstCh)
	}
}

func decodeBencode(bencode string) (any, error) {
	r := bufio.NewReader(strings.NewReader(bencode))

	return decodeValue(r)
}

// bencodeString encodes a string into a bencoded string.
// Example: spam -> 4:spam
func bencodeString(s string) string {
	return fmt.Sprintf("%d:%s", len(s), s)
}

// bencodeInt encodes an integer into a bencoded integer.
// Example: 42 -> i42e
func bencodeInt(i int) string {
	return fmt.Sprintf("i%ve", i)
}

// bencodeList encodes a list into a bencoded list.
// Example: ["spam", "eggs"] -> l4:spam
func bencodeList(l []any) (string, error) {
	var res strings.Builder

	res.WriteString("l")

	for _, v := range l {
		val, err := bencodeVal(v)
		if err != nil {
			return "", err
		}

		res.WriteString(val)

	}

	res.WriteString("e")

	return res.String(), nil
}

// bencodeDict encodes a dictionary into a bencoded dictionary.
// Example: {"spam": "eggs"} -> d4:spam4:eggse
func bencodeDict(d map[string]any) (string, error) {
	var res strings.Builder

	// Sort keys to ensure deterministic output
	// when encoding the dictionary
	keys := make([]string, 0, len(d))

	for k := range d {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	res.WriteString("d")

	for _, k := range keys {
		res.WriteString(bencodeString(k))

		v := d[k]
		val, err := bencodeVal(v)
		if err != nil {
			return "", err
		}

		res.WriteString(val)
	}

	res.WriteString("e")

	return res.String(), nil
}

// bencodeVal encodes a value into a bencoded value.
// The value can be a string, integer, list or dictionary.
func bencodeVal(v any) (string, error) {
	switch v := v.(type) {
	case string:
		return bencodeString(v), nil
	case []byte:
		return bencodeString(string(v)), nil
	case int:
		return bencodeInt(v), nil
	case []any:
		return bencodeList(v)
	case map[string]any:
		return bencodeDict(v)
	default:
		return "", fmt.Errorf("invalid data type %T, for val %v", v, v)
	}
}
