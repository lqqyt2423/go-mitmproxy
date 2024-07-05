package main

import "C"
import (
	"fmt"

	"github.com/lqqyt2423/go-mitmproxy/proxy"
)

//export Add
func Add(a, b int) int {
	return a + b
}

//export Greet
func Greet() {
	fmt.Println("Hello from Go!")

	o := proxy.Options{}
	fmt.Printf("opt: %v\n", o)
}

func main() {}
