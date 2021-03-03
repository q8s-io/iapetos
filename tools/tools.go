package tools

import (
	"strconv"
)

func StringToInt(str string) int {
	v, _ := strconv.Atoi(str)
	return v
}

func MatchStringFromArray(strArray []string, value string) bool {
	for _, v := range strArray {
		if v == value {
			return true
		}
	}
	return false
}

func RemoveString(strArray []string, value string) []string {
	for i, v := range strArray {
		if v == value {
			strArray = append(strArray[:i], strArray[i+1:]...)
			break
		}
	}
	return strArray
}

func IntToIntr32(index int) *int32 {
	v := int32(index)
	return &v
}
