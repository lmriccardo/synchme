package utils

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ascii defines the characters used for table borders.
// We only use single-line characters for simplicity without a dedicated double-line option.
const (
	hb  = "─" // horizontal border
	vb  = "│" // vertical border
	dt  = "┬" // down T
	ut  = "┴" // up T
	lt  = "┤" // left T
	rt  = "├" // right T
	adr = "┌" // angled down-right
	adl = "┐" // angled down-left
	aur = "└" // angled up-right
	aul = "┘" // angled up-left
	cr  = "┼" // cross
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

// ComputeSHA256 returns the SHA-256 of the input string
func ComputeSHA256(input string) (hashed string) {
	hash := sha256.New()
	hash.Write([]byte(input))
	hashed = fmt.Sprintf("%x", hash.Sum(nil))
	return
}

// calcColWidths computes the maximum text width for each column
// across all rows. Used to align the table columns properly.
func CalcColWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return nil
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	return widths
}

// makeTopBorder builds the top border line of the table,
// using ┌ ┬ ┐ characters to form corners and intersections.
func MakeTopBorder(widths []int) string {
	var sb strings.Builder
	sb.WriteString(adr)
	for i, w := range widths {
		sb.WriteString(strings.Repeat(hb, w+2))
		if i == len(widths)-1 {
			sb.WriteString(adl)
		} else {
			sb.WriteString(dt)
		}
	}
	return sb.String()
}

// makeMidBorder builds a separator line between header and body rows,
// using ├ ┼ ┤ to connect cells horizontally and vertically.
func MakeMidBorder(widths []int) string {
	var sb strings.Builder
	sb.WriteString(rt)
	for i, w := range widths {
		sb.WriteString(strings.Repeat(hb, w+2))
		if i == len(widths)-1 {
			sb.WriteString(lt)
		} else {
			sb.WriteString(cr)
		}
	}
	return sb.String()
}

// makeBottomBorder builds the bottom border line of the table,
// using └ ┴ ┘ characters for the corners and intersections.
func MakeBottomBorder(widths []int) string {
	var sb strings.Builder
	sb.WriteString(aur)
	for i, w := range widths {
		sb.WriteString(strings.Repeat(hb, w+2))
		if i == len(widths)-1 {
			sb.WriteString(aul)
		} else {
			sb.WriteString(ut)
		}
	}
	return sb.String()
}

// formatRow formats a single row of data cells into a Unicode-bordered line.
// It uses │ separators between columns and right-pads each cell to fit its width.
func FormatRow(cells []string, widths []int) string {
	var sb strings.Builder
	sb.WriteString(vb)
	for i, cell := range cells {
		sb.WriteString(" " + StringInsertPad(cell, 0, widths[i], " ") + " " + vb)
	}
	sb.WriteString("\n")
	return sb.String()
}
