package reader

import (
	"bufio"
	"os"
	"strings"
)

func ReadStdin() string {
	scanner := bufio.NewScanner(os.Stdin)
	var result []string
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	return strings.Join(result, "")
}
