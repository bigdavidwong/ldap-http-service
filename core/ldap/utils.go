package ldap

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-ldap/ldap"
	"ldap-http-service/lib/ers"
	"ldap-http-service/lib/utils"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func formatSID(sidBytes []byte) (string, error) {
	if len(sidBytes) < 8 {
		return "", errors.New("invalid SID length")
	}

	revision := int(sidBytes[0])
	subAuthorityCount := int(sidBytes[1])
	identifierAuthority := binary.BigEndian.Uint64(append(make([]byte, 2), sidBytes[2:8]...))

	sid := fmt.Sprintf("S-%d-%d", revision, identifierAuthority)
	offset := 8
	for i := 0; i < subAuthorityCount; i++ {
		if len(sidBytes) < offset+4 {
			return "", errors.New("invalid sub authority data")
		}

		subAuthority := binary.LittleEndian.Uint32(sidBytes[offset : offset+4])
		sid = fmt.Sprintf("%s-%d", sid, subAuthority)
		offset += 4
	}

	return sid, nil
}

func formatGUID(guidBytes []byte) (string, error) {
	if len(guidBytes) != 16 {
		return "", fmt.Errorf("invalid GUID length: expected 16, got %d", len(guidBytes))
	}
	guid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", binary.LittleEndian.Uint32(guidBytes[0:4]), binary.LittleEndian.Uint16(guidBytes[4:6]), binary.LittleEndian.Uint16(guidBytes[6:8]), binary.BigEndian.Uint16(guidBytes[8:10]), guidBytes[10:])

	return guid, nil
}

func unFormatGUID(guidStr string) (string, error) {
	// 去掉破折号
	noDash := strings.ReplaceAll(guidStr, "-", "")

	// 字符串转为字节
	bytes, err := hex.DecodeString(noDash)
	if err != nil {
		return "", fmt.Errorf("hex decode string error: %v", err)
	}

	// 格式转换
	b := []byte{bytes[3], bytes[2], bytes[1], bytes[0], bytes[5], bytes[4], bytes[7], bytes[6], bytes[8], bytes[9], bytes[10], bytes[11], bytes[12], bytes[13], bytes[14], bytes[15]}

	return string(b), nil
}

func parseGeneralizedTime(generalizedTime string) (GeneralizedTime, error) {
	layout := "20060102150405.0Z"
	parsedTime, err := time.Parse(layout, generalizedTime)
	if err != nil {
		return GeneralizedTime{}, err
	}
	return GeneralizedTime(parsedTime), nil
}

func parseFileTime(fileTime string) (FileTime, error) {
	zeroTime := FileTime(time.Time{})
	// 如果是预设的最大值，则代表其永久生效，返回初始时间值
	if fileTime == "9223372036854775807" {
		return zeroTime, nil
	}
	ftInt, err := strconv.ParseInt(fileTime, 10, 64)
	if err != nil {
		return zeroTime, err
	}

	if ftInt > 1 {
		win32Epoch := time.Date(1601, time.January, 1, 0, 0, 0, 0, time.UTC)
		dt, err := utils.AddBigDur(win32Epoch, fileTime+"00")
		if err != nil {
			return zeroTime, err
		}
		return FileTime(dt), nil
	} else {
		return zeroTime, nil
	}
}

func setLdapAttr(name string, field reflect.Value, attr *ldap.EntryAttribute) error {
	if field.IsValid() && field.CanSet() {
		switch field.Kind() {
		case reflect.String:
			switch name {
			case "ObjectSid":
				sidStr, err := formatSID(attr.ByteValues[0])
				if err != nil {
					return &ers.OptErr{Option: "formatting ObjectSid", Message: err.Error()}
				}
				field.SetString(sidStr)
			case "ObjectGUID":
				guidStr, err := formatGUID(attr.ByteValues[0])
				if err != nil {
					return &ers.OptErr{Option: "formatting ObjectGUID", Message: err.Error()}
				}
				field.SetString(guidStr)
			default:
				field.SetString(attr.Values[0]) // 如果字段是字符串类型，则设置字符串值
			}
		case reflect.Int8, reflect.Int64:
			val, err := strconv.ParseInt(attr.Values[0], 10, 64) // 解析整数值
			if err == nil {
				field.SetInt(val) // 如果解析成功，则设置整数值
			}
		case reflect.Float64:
			val, err := strconv.ParseFloat(attr.Values[0], 64) // 解析浮点数值
			if err == nil {
				field.SetFloat(val) // 如果解析成功，则设置浮点数值
			}
		case reflect.Bool:
			val, err := strconv.ParseBool(attr.Values[0])
			if err == nil {
				field.SetBool(val)
			}
		case reflect.Struct:
			if field.Type() == reflect.TypeOf(GeneralizedTime{}) {
				val, err := parseGeneralizedTime(attr.Values[0])
				if err == nil {
					field.Set(reflect.ValueOf(val))
				}
			} else if field.Type() == reflect.TypeOf(FileTime{}) {
				val, err := parseFileTime(attr.Values[0])
				if err == nil {
					field.Set(reflect.ValueOf(val))
				}
			}
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.String {
				strSlice := make([]string, len(attr.Values))
				copy(strSlice, attr.Values)
				field.Set(reflect.ValueOf(strSlice))
			}
		}
	}

	return nil
}
