package templates

import (
	"mime"
	"net/url"
	"path/filepath"
	"unicode/utf8"
)

func fileDisplayName(name string) string {
	return filepath.Base(name)
}

func fileContentType(name string) string {
	ctype := mime.TypeByExtension(filepath.Ext(name))
	if ctype != "" {
		return ctype
	}
	return "application/octet-stream"
}

// var esc*, func isInCharacterRange and xmlEscapeText are modified from encoding/xml
// Original copyright: Copyright 2009 The Go Authors. All rights reserved.
// Original license: BSD-3-Clause (https://pkg.go.dev/golang.org/x/net/webdav?tab=licenses)

var (
	escQuot = []byte("&#34;") // shorter than "&quot;"
	escApos = []byte("&#39;") // shorter than "&apos;"
	escAmp  = []byte("&amp;")
	escLT   = []byte("&lt;")
	escGT   = []byte("&gt;")
	escTab  = []byte("&#x9;")
	escNL   = []byte("&#xA;")
	escCR   = []byte("&#xD;")
	escFFFD = []byte("\uFFFD") // Unicode replacement character
)

// Decide whether the given rune is in the XML Character Range, per
// the Char production of https://www.xml.com/axml/testaxml.htm,
// Section 2.2 Characters.
func isInCharacterRange(r rune) (inrange bool) {
	return r == 0x09 ||
		r == 0x0A ||
		r == 0x0D ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}

func escapeWebdavHref(input string) (output string) {
	s := []byte(input)
	var esc []byte

	last := 0
	for i := 0; i < len(input); {
		r, width := utf8.DecodeRune(s[i:])
		i += width
		switch r {
		case '"':
			esc = escQuot
		case '\'':
			esc = escApos
		case '&':
			esc = escAmp
		case '<':
			esc = escLT
		case '>':
			esc = escGT
		case '\t':
			esc = escTab
		case '\n':
			esc = escNL
		case '\r':
			esc = escCR
		default:
			if !isInCharacterRange(r) || (r == 0xFFFD && width == 1) {
				esc = escFFFD
				break
			}
			continue
		}
		output += string(s[last : i-width])
		output += string(esc)
		last = i
	}
	output += string(s[last:])
	return (&url.URL{Path: output}).EscapedPath()
}
