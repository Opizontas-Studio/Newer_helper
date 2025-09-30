package utils

import (
	"log"
	"strconv"
	"strings"
)

// ParseHexColor parses a hex color string (like "#FACF24") into an integer for Discord embeds.
// Returns the default red color (0xff0000) if parsing fails.
func ParseHexColor(hexColor string) int {
	if hexColor == "" {
		return 0xff0000 // Default red color
	}

	// Remove the # prefix if present
	hexColor = strings.TrimPrefix(hexColor, "#")

	// Parse the hex string as an integer
	colorInt, err := strconv.ParseInt(hexColor, 16, 64)
	if err != nil {
		log.Printf("Failed to parse hex color '%s': %v", hexColor, err)
		return 0xff0000 // Default red color
	}

	return int(colorInt)
}
