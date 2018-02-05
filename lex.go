/* Copyright (C) 2015 by Alexandru Cojocaru */

/* This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>. */

package main

import (
	"strings"
	"unicode/utf8"
)

const eof rune = -1

type lex struct {
	s     string
	p     int
	width int
	err   error
}

func newLex(s string) *lex {
	return &lex{s, 0, 0, nil}
}

func (l *lex) next() (r rune) {
	if l.p >= len(l.s) {
		return eof
	}
	r, l.width = utf8.DecodeRuneInString(l.s[l.p:])
	l.p += l.width
	return
}

func (l *lex) LineNumber() int {
	return 1 + strings.Count(l.s[:l.p], "\n")
}

func (l *lex) ColumnNumber() int {
	return l.p - strings.LastIndex(l.s[:l.p], "\n")
}

func (l *lex) Err() error {
	return l.err
}

func (l *lex) setErr(err error) {
	if l.err == nil {
		l.err = err
	}
}

func (l *lex) match(m string) bool {
	if !strings.HasPrefix(l.s[l.p:], m) {
		return false
	}

	l.p += len(m)
	return true
}

/*
func expect(l *lex, sep string) bool {
	if l.err != nil {
		return false
	}

	if !l.match(sep) {
		r := l.next()
		if r == eof {
			//			setErr(fmt.Errorf("expected %q but got EOF", sep))
			l.setErr(io.EOF)
			return false
		} else {
			l.setErr(fmt.Errorf("%d: expected %q but got %q", l.ColumnNumber()-1, sep, r))
			return false
		}
	}
	return true
}
*/

func (l *lex) span(m string) (string, bool) {
	i := strings.Index(l.s[l.p:], m)
	if i < 0 {
		return "", false
	}
	s := l.s[l.p : l.p+i]
	l.p += i + len(m)
	return s, true
}

func (l *lex) spanAny(chars string) (string, bool) {
	i := strings.IndexAny(l.s[l.p:], chars)
	if i < 0 {
		return "", false
	}
	s := l.s[l.p : l.p+i]
	l.p += i + len(chars)
	return s, true
}