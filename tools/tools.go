package tools

import (
	"strconv"
)

func StrToInt(str string) int {
	v, _ := strconv.Atoi(str)
	return v
}
