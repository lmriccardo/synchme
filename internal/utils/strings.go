package utils

import (
	"strings"
)

// StringInsertPad repeats a padding string a specified number of times on the
// left and right sides of the input string 's'.
func StringInsertPad(s string, l_size, r_size int, pad_s string) string {
	return strings.Repeat(pad_s, l_size) + s + strings.Repeat(pad_s, r_size)
}

// StringJustifyL left-justifies the input string 'content' within the specified
// total 'width' by appending spaces to the right.
//
// If the length of 'content' is greater than or equal to 'width', the original
// string is returned unchanged. Otherwise, the string is padded with spaces
// on the right to meet the desired width.
func StringJustifyL(content string, width int) string {
	content_len := len(content)
	if content_len >= width {
		return content
	}

	return StringInsertPad(content, 0, width-content_len, " ")
}

// StringJustifyR right-justifies the input string 'content' within the specified
// total 'width' by prepending spaces to the left.
//
// If the length of 'content' is greater than or equal to 'width', the original
// string is returned unchanged. Otherwise, the string is padded with spaces
// on the left to meet the desired width.
func StringJustifyR(content string, width int) string {
	content_len := len(content)
	if content_len >= width {
		return content
	}

	return StringInsertPad(content, width-content_len, 0, " ")
}

// StringCenter pads a string with spaces on both sides to achieve the specified total width.
// If the total padding is an odd number, the extra space is added to the right side.
func StringCenter(content string, width int) string {
	content_len := len(content)
	if content_len >= width {
		return content
	}

	// Calculate left padding (using integer division to drop any remainder)
	left_pad := (width-content_len)/2 + content_len

	// 1. Right-justify (pads the left) to the width of the left pad + content.
	// 2. Left-justify the result (pads the right) to the final total width.
	return StringJustifyL(StringJustifyR(content, left_pad), width)
}
