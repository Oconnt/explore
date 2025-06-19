package utils

import (
	"explore/pkg/proc/desc"
	"fmt"
)

func PrintVariable(v *desc.Variable) {
	if v == nil {
		return
	}

	fmt.Printf("%s: %s\n", v.Name, v.MultilineString("", ""))
}

func PrintStringLine(s ...string) {
	for _, str := range s {
		fmt.Println(str)
	}
}

func PrintBytes(bs []byte) {

}
