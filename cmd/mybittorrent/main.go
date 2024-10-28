package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func decodeBencode(bencodedString string) (interface{}, error) {
	firstCh := rune(bencodedString[0])

	// Example:
	// - 5:hello -> hello
	// - 10:hello12345 -> hello12345
	if unicode.IsDigit(firstCh) {
		firstColonIdx := strings.Index(bencodedString, ":")

		lengthStr := bencodedString[:firstColonIdx]
		length, err := strconv.Atoi(lengthStr)
		if err != nil {
			return "", err
		}

		return bencodedString[firstColonIdx+1 : firstColonIdx+1+length], nil
	} else if firstCh == 'i' {
		// Example:
		// - i123e -> 123
		// - i-123e -> -123
		endIntIndex := strings.Index(bencodedString, "e")

		if endIntIndex == -1 {
			return "", fmt.Errorf("Invalid integer")
		}

		numStr := bencodedString[1:endIntIndex]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return "", err
		}

		return num, nil
	}

	return "", fmt.Errorf("Only strings are supported at the moment")
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
