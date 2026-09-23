//go:build ignore

package main

import (
	"fmt"
	"time"

	pdf "github.com/dslipak/pdf"
)

func main() {
	path := "/Users/jake/Documents/books/The Iliad -- Homer (trans_ Robert Fagles) -- 2009 -- 01fc44f7c1e5ff653a972777d17da906 -- Anna’s Archive.pdf"
	t0 := time.Now()
	r, err := pdf.Open(path)
	fmt.Printf("open: %v err=%v\n", time.Since(t0), err)
	if err != nil {
		return
	}
	n := r.NumPage()
	fmt.Printf("pages: %d\n", n)
	t1 := time.Now()
	totalRows := 0
	for p := 1; p <= n; p++ {
		pt0 := time.Now()
		rows, err := r.Page(p).GetTextByRow()
		el := time.Since(pt0)
		if err != nil {
			fmt.Printf("page %d err %v\n", p, err)
			continue
		}
		totalRows += len(rows)
		if el > 500*time.Millisecond || p <= 3 || p == n {
			fmt.Printf("page %d: %v (%d rows)\n", p, el, len(rows))
		}
		if time.Since(t1) > 90*time.Second {
			fmt.Printf("STOPPED after 90s at page %d/%d\n", p, n)
			return
		}
	}
	fmt.Printf("all pages: %v totalRows=%d\n", time.Since(t1), totalRows)
}
