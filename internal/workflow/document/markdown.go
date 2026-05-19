package document

import (
	"strconv"
	"strings"
)

func AssembleMarkdown(pages []OCRPage) string {
	parts := make([]string, 0, len(pages))
	for i, page := range pages {
		name := page.Name
		if name == "" {
			name = "page-" + strconv.Itoa(i+1)
		}
		parts = append(parts, "<!-- Page "+strconv.Itoa(i+1)+" "+name+" -->\n"+page.Text)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func zeroPad(value int) string {
	if value < 10 {
		return "000" + strconv.Itoa(value)
	}
	if value < 100 {
		return "00" + strconv.Itoa(value)
	}
	if value < 1000 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
