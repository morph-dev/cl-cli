package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/log"
)

func InitLogger(loglevel int) {
	verbosity := log.FromLegacyLevel(loglevel)
	handler := log.NewTerminalHandlerWithLevel(os.Stdout, verbosity /* useColor= */, true)
	log.SetDefault(log.NewLogger(handler))
}

func PrintJson(prefix string, value any) {
	if output, err := json.MarshalIndent(value, "", "  "); err != nil {
		log.Error("Error marshaling into json", "err", err)
	} else {
		log.Info(prefix + "\n" + string(output))
	}
}

func PromptBool(message string) bool {
	fmt.Printf("%s (y/n): ", message)

	var answer string
	fmt.Scan(&answer)

	return strings.ToLower(answer) == "y"
}
