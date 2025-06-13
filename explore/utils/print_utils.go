package utils

import (
	"fmt"
	"github.com/go-delve/delve/service/api"
)

func PrintVariable(v *api.Variable) {
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
