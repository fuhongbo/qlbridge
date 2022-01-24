/*
Package driver registers a QL Bridge sql/driver named "qlbridge"

Usage

	package main

	import (
		"database/sql"
		_ "github.com/fuhongbo/qlbridge/qlbdriver"
	)

	func main() {

		db, err := sql.Open("qlbridge", "csv:///dev/stdin")
		if err != nil {
			log.Fatal(err)
		}

		// Use db here

	}

*/
package qlbdriver

import "github.com/fuhongbo/qlbridge/exec"

func init() {
	exec.RegisterSqlDriver()
}
