package ers

import (
	"fmt"
	"net/http"
	"time"
)

// CustomErr 定义自定义错误接口
type CustomErr interface {
	Error() string
	Code() int
	HttpCode() int
}

// BaseErr 定义自定义错误基础类型
type BaseErr struct{}

func (e *BaseErr) Code() int {
	return 1000
}

func (e *BaseErr) HttpCode() int {
	return 500
}

func (e *BaseErr) Error() string {
	return "internal server error"
}

type SystemErr struct {
	BaseErr
	Message string
}

func (e *SystemErr) Error() string {
	return fmt.Sprintf("interval server error: %s", e.Message)
}

type OptErr struct {
	BaseErr
	Option  string
	Message string
}

func (e *OptErr) Error() string {
	return fmt.Sprintf("failed to %s: %s", e.Option, e.Message)
}

// TimeoutErr 超时异常
type TimeoutErr struct {
	BaseErr
	Option string
	Time   time.Duration
}

func (e *TimeoutErr) Error() string {
	return fmt.Sprintf("%s timeout after %vs", e.Option, e.Time.Seconds())
}

// NotFoundError 访问对象不存在异常
type NotFoundError struct {
	BaseErr
	Object string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no object matched '%s'", e.Object)
}

func (e *NotFoundError) HttpCode() int {
	return http.StatusNotFound
}

func (e *NotFoundError) Code() int {
	return 96
}

// ObjExistError 对象已存在，冲突错误
type ObjExistError struct {
	BaseErr
	Object string
}

func (e *ObjExistError) Error() string {
	return fmt.Sprintf("object already exists err: %s", e.Object)
}

func (e *ObjExistError) HttpCode() int {
	return http.StatusConflict
}

func (e *ObjExistError) Code() int {
	return 68
}

// InvalidJsonErr Json请求体异常
type InvalidJsonErr struct {
	BaseErr
}

func (e *InvalidJsonErr) HttpCode() int {
	return http.StatusBadRequest
}

func (e *InvalidJsonErr) Error() string {
	return "invalid json body"
}

// UnSupportedErr 不支持的对象或属性异常
type UnSupportedErr struct {
	BaseErr
	Object     string
	ObjectType string
}

func (e *UnSupportedErr) Error() string {
	return fmt.Sprintf("unsupported %s: '%s'", e.ObjectType, e.Object)
}

// InvalidFormatErr 格式错误
type InvalidFormatErr struct {
	BaseErr
	Name   string
	Object string
}

func (e *InvalidFormatErr) Error() string {
	return fmt.Sprintf("invalid %s format: '%s'", e.Name, e.Object)
}

// ForbiddenErr 禁止的操作异常
type ForbiddenErr struct {
	BaseErr
	Message string
}

func (e *ForbiddenErr) HttpCode() int {
	return http.StatusForbidden
}

func (e *ForbiddenErr) Error() string {
	return e.Message
}
