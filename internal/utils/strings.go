package utils

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ascii defines the characters used for table borders.
// We only use single-line characters for simplicity without a dedicated double-line option.
const (
	hb = "─" // horizontal border
	vb = "│" // vertical border
	tl = "┌" // top-left corner
	tr = "┐" // top-right corner
	bl = "└" // bottom-left corner
	br = "┘" // bottom-right corner
	tj = "┬" // top join
	bj = "┴" // bottom join
	lj = "├" // left join
	rj = "┤" // right join
	cj = "┼" // center join
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
			if len([]rune(cell)) > widths[i] {
				widths[i] = len([]rune(cell))
			}
		}
	}
	return widths
}

// border constructs any horizontal border (top, middle, bottom)
func makeBorder(left, join, right string, widths []int) string {
	var sb strings.Builder
	sb.WriteString(left)
	for i, w := range widths {
		sb.WriteString(strings.Repeat(hb, w+2))
		if i == len(widths)-1 {
			sb.WriteString(right)
		} else {
			sb.WriteString(join)
		}
	}
	return sb.String()
}

func MakeTopBorder(widths []int) string    { return makeBorder(tl, tj, tr, widths) }
func MakeMidBorder(widths []int) string    { return makeBorder(lj, cj, rj, widths) }
func MakeBottomBorder(widths []int) string { return makeBorder(bl, bj, br, widths) }

// FormatRow formats one row with proper padding
func FormatRow(cells []string, widths []int) string {
	var sb strings.Builder
	sb.WriteString(vb)
	for i, cell := range cells {
		cellRunes := []rune(cell)
		padding := widths[i] - len(cellRunes)
		sb.WriteString(" " + cell + strings.Repeat(" ", padding) + " " + vb)
	}
	sb.WriteString("\n")
	return sb.String()
}
