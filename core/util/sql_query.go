/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package util

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type SQLQueryString struct {
	query   string
	lowered string
}

func NewSQLQueryString(query string) *SQLQueryString {
	query = strings.TrimSpace(query)
	return &SQLQueryString{
		query:   query,
		lowered: strings.ToLower(query),
	}
}

func (p SQLQueryString) AfterAll(word string) (atAfters []string) {
	indices := regexp.MustCompile(strings.ToLower(word)).
		FindAllStringIndex(p.lowered, -1)
	for _, index := range indices {
		atAfters = append(atAfters, p.after(index[1]))
	}
	return
}

// TableNames returns all table names of the SQL statement
func (p SQLQueryString) TableNames() (names []string, err error) {
	firstSyntax := p.lowered[:strings.IndexRune(p.lowered, ' ')]

	switch firstSyntax {
	case "select":
		names = append(names, p.tableNamesByFROM()...)
		names = append(names, p.AfterAll("join")...)
	default:
		err = fmt.Errorf("unsupport sql statment %s", firstSyntax)
	}
	return
}

func (p SQLQueryString) tableNamesByFROM() (names []string) {
	indices := regexp.MustCompile("from(.*?)(left|inner|right|outer|full)|from(.*?)join|from(.*?)where|from(.*?);|from(.*?)$").
		FindAllStringIndex(p.lowered, -1)

	for _, index := range indices {
		fromStmt := p.lowered[index[0]:index[1]]
		lastSyntax := fromStmt[strings.LastIndex(fromStmt, " ")+1:]

		var tableStmt string
		if lastSyntax == "from" || lastSyntax == "where" || lastSyntax == "left" ||
			lastSyntax == "right" || lastSyntax == "join" || lastSyntax == "inner" ||
			lastSyntax == "outer" || lastSyntax == "full" {
			tableStmt = p.query[index[0]+len("from")+1 : index[1]-len(lastSyntax)-1]
		} else {
			tableStmt = p.query[index[0]+len("from")+1:]
		}
		if strings.Contains(strings.ToLower(tableStmt), "from") {
			subP := NewSQLQueryString(tableStmt)
			names = append(names, subP.tableNamesByFROM()...)
			continue
		}

		for _, name := range strings.Split(tableStmt, ",") {
			names = append(names, cleanName(name))
		}
	}
	return
}

func cleanName(name string) string {
	name = strings.Fields(name)[0]
	name = strings.TrimSpace(name)
	name = strings.Trim(name,"`")
	lastRune := name[len(name)-1]
	if lastRune == ';' {
		name = name[:len(name)-1]
	}
	return name
}


func (p SQLQueryString) after(iWord int) (atAfter string) {
	iAfter := 0
	for i := iWord; i < len(p.lowered); i++ {
		r := rune(p.lowered[i])
		if unicode.IsLetter(r) && iAfter <= 0 {
			iAfter = i
		}
		if (unicode.IsSpace(r) || unicode.IsPunct(r)) && iAfter > 0 {
			atAfter = p.query[iAfter:i]
			break
		}
	}

	if atAfter == "" {
		atAfter = p.query[iAfter:]
	}

	return
}
