package reader

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
)

// ReadStdin reads all stuff coming from stdin.
func ReadStdin() string {
	var f *os.File
	f = os.Stdin
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("%v", err)
		}
	}()

	scanner := bufio.NewScanner(f)
	var result []string
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	return concat(result)
}

func concat(s []string) string {
	var buffer bytes.Buffer
	for _, v := range s {
		buffer.WriteString(v)
	}
	return buffer.String()
}
