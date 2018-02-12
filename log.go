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

// Package logparse parses a log entry in the most common formats.

package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const timeLayout = "02/Jan/2006:15:04:05 -0700"

// An Entry represents a log entry.
type Entry struct {
	// The IP of the client which made the request (nil if unknown).
	Host net.IP
	// The username of the logged in user making the request (empty if anonymous).
	User string
	// The time the request was made (zero value if unknown, check with IsZero).
	Time time.Time
	// The HTTP request line from the client (nil if unknown).
	Request *http.Request
	// The HTTP status code returned to the client (-1 if unknown).
	Status int
	// The size in bytes of the data sent to the client (0 if no data sent).
	Bytes int
	// The URL of the host the client comes from (nil if unknown).
	Referer *url.URL
	// The user agent of the client (nil if unknown).
	UserAgent string
	// X-Forwarded-For
	XForwardedFor string
	// Response time
	ResponseTime float64
}

// Formats the Entry e in the combined log format.
func (e *Entry) String() string {
	s := ""

	if e.Host == nil {
		s += "-"
	} else {
		s += e.Host.String()
	}
	s += " "

	s += "- "

	if e.User == "" {
		s += "-"
	} else {
		s += e.User
	}
	s += " "

	if e.Time.IsZero() {
		s += "-"
	} else {
		s += "[" + e.Time.Format(timeLayout) + "]"
	}
	s += " "

	if e.Request == nil {
		s += "-"
	} else {
		s += fmt.Sprintf(`"%s %s %s"`, e.Request.Method, e.Request.URL.RequestURI(), e.Request.Proto)
	}
	s += " "

	if e.Status < 0 {
		s += "-"
	} else {
		s += strconv.Itoa(e.Status)
	}
	s += " "

	s += strconv.Itoa(e.Bytes)
	s += " "

	if e.Referer == nil {
		s += "-"
	} else {
		s += `"` + e.Referer.String() + `"`
	}
	s += " "

	if e.UserAgent == "" {
		s += "-"
	} else {
		s += `"` + e.UserAgent + `"`
	}

	if e.ResponseTime == 0 {
		s += "xyz"
	}

	return s
}

func nextField(l *lex, sep string) (string, error) {
	f, ok := l.span(sep)
	if !ok {
		return "", fmt.Errorf("%q: cannot find separator %q in line:\n %s \n", l.s[l.p:], sep, l.s)
	}
	return f, nil
}

func expect(l *lex, sep rune) error {
	if !l.match(string(sep)) {
		r := l.next()
		if r == eof {
			return fmt.Errorf("expected %q but got EOF", sep)
		} else {
			return fmt.Errorf("%d: expected %q but got %q", l.ColumnNumber()-1, sep, r)
		}
	}
	return nil
}

// FIXME: error checks are too noisy
func common(l *lex) (*Entry, error) {
	e := &Entry{}

	ip, err := nextField(l, " ")
	if err != nil {
		return nil, err
	}
	e.Host = net.ParseIP(ip)
	if e.Host == nil {
		return nil, fmt.Errorf("cannot parse IP %q", ip)
	}

	_, err = nextField(l, " ")
	if err != nil {
		return nil, err
	}

	e.User, err = nextField(l, " ")
	if err != nil {
		return nil, err
	}

	err = expect(l, '[')
	if err != nil {
		return nil, err
	}
	t, err := nextField(l, "] ")
	if err != nil {
		return nil, err
	}
	e.Time, err = time.Parse(timeLayout, t)
	if err != nil {
		return nil, err
	}

	err = expect(l, '"')
	if err != nil {
		return nil, err
	}
	r, err := nextField(l, `" `)
	if err != nil {
		return nil, err
	}
	e.Request, err = http.ReadRequest(bufio.NewReader(strings.NewReader(r + "\r\n\r\n")))
	if err != nil {
		return nil, err
	}

	s, err := nextField(l, " ")
	if err != nil {
		return nil, err
	}
	e.Status, err = strconv.Atoi(s)
	if err != nil {
		return nil, err
	}

	b, err := nextField(l, " ")
	if err != nil {
		b = l.s[l.p:]
		//		return nil, err
	}
	e.Bytes, err = strconv.Atoi(b)
	if err != nil {
		return nil, err
	}

	return e, nil
}

// Common parses a log line containing a log entry in the common log format.
//
// An entry in the common log format has the form:
//  Host - User Time Request Status Bytes
// where:
//  Host is the ip of the client which made the request.
//  - this field never is used.
//  User is the name of the logged in user doing the request.
//  Time is the date/time/zone the request was made.
//  Request is the HTTP request line from the client.
//  Status is the status code returned to the client.
//  Bytes is the size in bytes of the data sent to the client.
func Common(line string) (*Entry, error) {
	l := newLex(line)
	e, err := common(l)
	return e, err
}

// Combined parses a log line containing a log entry in the combined log format.
//
// An entry in the combined log format has the form:
//  Host - User Time Request Status Bytes Referer UserAgent
// basicaly it's the same as the common log format with the added fields:
//  Referer the URL of the host the client comes from
//  UserAgent the user agent of the client

// %SM Added ResponseTime at end

func Combined(line string) (*Entry, error) {
	l := newLex(line)
	e, err := common(l)
	if err != nil {
		return nil, err
	}

	err = expect(l, '"')
	if err != nil {
		return nil, err
	}
	ref, err := nextField(l, `" `)
	if err != nil {
		return nil, err
	}
	if ref != `-` {
		e.Referer, err = url.ParseRequestURI(ref)
		if err != nil {
			// Don't care if we get invalid URI here
			//			return nil, err
		}
	}

	err = expect(l, '"')
	if err != nil {
		return nil, err
	}
	uas, err := nextField(l, `"`)
	if err != nil {
		return nil, err
	}
	e.UserAgent = uas

	// Commenting out as getting other another way in the main loop
	/*
		// X-Forwarded-For - not used, but need to parse
		l.p++ // Add one; not clear why
		err = expect(l, '"')
		if err != nil {
			return nil, err
		}
		xff, err := nextField(l, `"`)
		if err != nil {
			return nil, err
		}
		e.XForwardedFor = xff


		// Get ResponseTime
		l.p++ // Add one; not clear why
		rt, err := nextField(l, " ")
		if err != nil {
			//return nil, err
			// If last field have to return text, not error as no final separator on line
			rt = l.s[l.p:]
		}
		if len(string(rt)) > 0 {
			e.ResponseTime, err = strconv.ParseFloat(rt, 64)
		}
		if err != nil {
			return nil, err
		}
	*/

	return e, nil
}

type ExtendedDirective struct {
}

func (x *ExtendedDirective) Extended(line string) *Entry {
	return nil
}
