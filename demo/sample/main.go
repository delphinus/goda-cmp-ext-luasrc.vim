package main

import (
	"fmt"
	"strings"
)

func main() {
	words := strings.Fields("the quick brown fox")
	fmt.Println(len(words))
}
