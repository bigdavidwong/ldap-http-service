package utils

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const MaxInt64 = int64(^uint64(0) >> 1)

// AddBigDur golang单次只能进行int64的时间计算，该方法用于分段递归计算大int类型的时间跨度
func AddBigDur(t time.Time, bigNumStr string) (time.Time, error) {
	dur := new(big.Int)
	_, ok := dur.SetString(bigNumStr, 10)
	if !ok {
		return time.Time{}, errors.New("failed to set big integer")
	}

	if dur.Cmp(big.NewInt(0).SetInt64(MaxInt64)) < 0 {
		return t.Add(time.Duration(dur.Int64())), nil
	}
	t = t.Add(time.Duration(MaxInt64))

	newDur := big.NewInt(0)
	newDur.Sub(dur, big.NewInt(0).SetInt64(MaxInt64))
	return AddBigDur(t, newDur.String())
}

func GenUuid(prefix string) string {
	Uuid, _ := uuid.NewRandom()
	return fmt.Sprintf("%s_%s", prefix, Uuid)
}

func IsStrongPassword(username, password string) bool {
	minLength := 7
	upper := false
	lower := false
	digit := false
	special := false
	count := 0

	// 检查长度
	if len(password) < minLength {
		return false
	}

	// 检查复杂性
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			upper = true
		case unicode.IsLower(char):
			lower = true
		case unicode.IsDigit(char):
			digit = true
		case isSpecialChar(char):
			special = true
		}
	}

	// 计算符合条件的类别数
	if upper {
		count++
	}
	if lower {
		count++
	}
	if digit {
		count++
	}
	if special {
		count++
	}

	// 检查密码中是否包含用户名的连续子字符串
	if containsSubstring(username, password) {
		return false
	}

	// 如果至少有3个类别符合条件，返回true
	return count >= 3
}

func InSlice(item string, slice []string) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

func RegexFirst(s, pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}

	match := re.FindString(s)
	return match
}

func InSliceIC(slice []string, item string) bool {
	item = strings.ToLower(item)
	for _, i := range slice {
		if strings.ToLower(i) == item {
			return true
		}
	}
	return false
}
