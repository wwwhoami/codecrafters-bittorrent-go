package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// decodeStr reads a string from the reader with the length
// specified by the integer that precedes the string with a colon.
// Example: 4:spam -> spam
func decodeStr(r *bufio.Reader) (string, error) {
	length, err := decodeInt(r, ':')
	if err != nil {
		return "", err
	} else if length < 0 {
		return "", fmt.Errorf("Invalid string length: %d", length)
	}

	if peekBuf, peekErr := r.Peek(length); peekErr != nil {
		_, err := r.Discard(length)
		return string(peekBuf), err
	}

	buf := make([]byte, length)

	_, err = r.Read(buf)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// decodeInt reads a number from the reader until the delimiter character is found.
// The delimiter character is not included in the returned string.
// Example: i42e -> 42
func decodeInt(r *bufio.Reader, delim byte) (int, error) {
	str, err := r.ReadString(delim)
	if err != nil {
		return 0, err
	}
	// Discard delim character
	str = str[:len(str)-1]

	return strconv.Atoi(str)
}

// decodeList reads a list from the reader until the 'e' character is found.
// The 'e' character is not included in the returned list.
// Example: l4:spam4:eggse -> ["spam", "eggs"]
func decodeList(r *bufio.Reader) (list []interface{}, err error) {
	list = make([]interface{}, 0)

	for {
		ch, err := r.Peek(1)
		if err != nil {
			return nil, err
		}

		if ch[0] == 'e' {
			// Discard 'e', proceeding reader by one byte
			if _, err := r.ReadByte(); err != nil {
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

func decodeDict(r *bufio.Reader) (dict map[string]interface{}, err error) {
	dict = make(map[string]interface{})

	for {
		ch, err := r.Peek(1)
		if err != nil {
			return nil, err
		}

		if ch[0] == 'e' {
			// Discard 'e', proceeding reader by one byte
			if _, err := r.ReadByte(); err != nil {
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
func decodeValue(r *bufio.Reader) (interface{}, error) {
	firstCh, _, err := r.ReadRune()
	if err != nil {
		return nil, err
	}

	switch {
	case unicode.IsDigit(firstCh) || firstCh == '-':
		// Unread first character to read the string
		if err = r.UnreadRune(); err != nil {
			return nil, err
		}

		return decodeStr(r)
	case firstCh == 'i':
		return decodeInt(r, 'e')
	case firstCh == 'l':
		return decodeList(r)
	case firstCh == 'd':
		return decodeDict(r)
	default:
		return "", fmt.Errorf("Invalid data type: %c", firstCh)
	}
}

func decodeBencode(bencode string) (interface{}, error) {
	r := bufio.NewReader(strings.NewReader(bencode))

	return decodeValue(r)
}

func main() {
	command := os.Args[1]

	if command == "decode" {

		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
