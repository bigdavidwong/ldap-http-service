package ldap

import (
	"fmt"
	"github.com/go-ldap/ldap"
	"ldap-http-service/lib/ers"
	"ldap-http-service/lib/utils"
	"reflect"
	"strings"
	"time"
)

// FileTime 定义结构体用于Windows中Integer8类型的时间字段的处理
type FileTime time.Time

func (ft FileTime) MarshalJSON() ([]byte, error) {
	t := time.Time(ft)
	stamp := fmt.Sprintf("\"%s\"", t.Format(time.RFC3339))
	return []byte(stamp), nil
}

// GeneralizedTime 定义结构体用于Windows中GeneralizedTime类型的时间字段的处理
type GeneralizedTime time.Time

func (gt GeneralizedTime) MarshalJSON() ([]byte, error) {
	t := time.Time(gt)
	stamp := fmt.Sprintf("\"%s\"", t.Format(time.RFC3339))
	return []byte(stamp), nil
}

// BaseObject ldap对象基础结构体，包含AD域中ldap对象通用的ldap属性
type BaseObject struct {
	Name              string          `ldap:"name" json:"name"`
	DisplayName       string          `ldap:"displayName" json:"displayName"`
	SAMAccountName    string          `ldap:"sAMAccountName" json:"sAMAccountName"`
	DistinguishedName string          `ldap:"distinguishedName" json:"distinguishedName"`
	Description       string          `ldap:"description" json:"description"`
	WhenCreated       GeneralizedTime `ldap:"whenCreated" json:"whenCreated"`
	WhenChanged       GeneralizedTime `ldap:"whenChanged" json:"whenChanged"`
	ObjectClass       []string        `ldap:"objectClass" json:"objectClass"`
	ObjectCategory    string          `ldap:"objectCategory" json:"objectCategory"`
	ObjectGUID        string          `ldap:"objectGUID" json:"objectGUID"`
	ObjectSid         string          `ldap:"objectSid" json:"objectSid"`
}

// SpecObject 定义接口，表示具体的Ldap对象：如用户、群组、计算机等，所有具体的Ldap对象类型都需要嵌套BaseObject
type SpecObject interface {
	GetSpecType() reflect.Type
	ReturnBaseObj() *BaseObject
}

func (b *BaseObject) GetSpecType() reflect.Type {
	return reflect.TypeOf(*b)
}

func (b *BaseObject) ReturnBaseObj() *BaseObject {
	return b
}

// 校验sAMAccountName是否可用
func (l *ldapConnPool) checkAvailability(name string) (bool, BaseObject) {
	// 如果为空，则返回异常
	if name == "" {
		panic(&ers.ForbiddenErr{Message: "name cannot be an empty string"})
	}

	// 禁止系统预留的名字
	var fbdName = []string{"service", "network service", "local service", "local system", "network", "local"}
	if utils.InSlice(name, fbdName) {
		panic(&ers.ForbiddenErr{Message: "this name is reserved for the system"})
	}

	// ldap搜索是否存在匹配的邮箱地址前缀或sAMAccountName
	filter := fmt.Sprintf("(sAMAccountName=%s)", name)
	for _, domain := range l.Zones {
		filter = filter + fmt.Sprintf("(proxyAddresses=smtp:%s@%s)", name, domain)
	}
	filter = fmt.Sprintf("(|%s)", filter)

	var obj BaseObject
	err := l.searchLdapObject(&obj, filter, "")
	// 如果获得了不存在错误，则代表其可用
	if _, ok := err.(*ers.NotFoundError); ok {
		return true, obj
	}

	// 如果存在其他错误，则引发系统异常
	if err != nil {
		panic(err)
	}

	// 如果没有错误，代表找到了已有对象，触发已被使用异常
	return false, obj
}

// 移动对象到OU
func (l *ldapConnPool) moveObjectToOU(dn, newOU string) error {
	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	// 分离原始 DN 的 CN 和 OU 部分
	parts := strings.SplitN(dn, ",", 2)
	if len(parts) != 2 {
		return &ers.InvalidFormatErr{Name: "DN", Object: dn}
	}
	cn := parts[0]

	// 修改操作
	modDNReq := ldap.NewModifyDNRequest(dn, cn, true, newOU)
	if err = conn.ModifyDN(modDNReq); err != nil {
		return &ers.OptErr{Option: fmt.Sprintf("moving '%s' to OU '%s'", dn, newOU), Message: err.Error()}
	}
	return nil
}

// 修改对象
func (l *ldapConnPool) modifyObj(dn string, replaceAttr map[string][]string) error {
	if len(replaceAttr) == 0 {
		return &ers.OptErr{Option: fmt.Sprintf("modify obj '%s'", dn), Message: "no valid field in replaceAttr"}
	}
	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	modReq := ldap.NewModifyRequest(dn, []ldap.Control{})
	for k, v := range replaceAttr {
		modReq.Replace(k, v)
	}
	if err = conn.Modify(modReq); err != nil {
		return &ers.OptErr{Option: fmt.Sprintf("modify obj '%s'", dn), Message: err.Error()}
	}
	return nil
}

// 通用方法，通过指定的Object类型，和搜索过滤条件，返回查找的Ldap对象结构体；
func (l *ldapConnPool) searchLdapObject(obj SpecObject, filter string, ou string) error {
	// 如果没有传入搜索OU，则全局搜索
	if ou == "" {
		ou = l.BaseDN
	}

	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	// 获取Value的类型信息（即结构体的类型）
	objType := obj.GetSpecType()
	baseObjType := reflect.TypeOf(*obj.ReturnBaseObj())

	// 初始化一个空字符串切片，用于存储LDAP属性
	var searchAttr []string

	// 先增加通用的Object字段
	for i := 0; i < baseObjType.NumField(); i++ {
		field := baseObjType.Field(i)
		tag := field.Tag.Get("ldap")
		searchAttr = append(searchAttr, tag)
	}

	// 遍历结构体的所有字段
	for i := 0; i < objType.NumField(); i++ {
		// 获取字段的LDAP标签值
		field := objType.Field(i)
		tag := field.Tag.Get("ldap")
		// 如果标签值不为空，则将其添加到searchAttr切片中
		if tag != "" && tag != "member" {
			searchAttr = append(searchAttr, tag)
		}
	}

	searchRequest := ldap.NewSearchRequest(
		ou,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		searchAttr, // 使用从结构体标签生成的搜索属性列表
		nil,
	)
	sr, err := conn.Search(searchRequest)
	if err != nil {
		return err
	}

	if len(sr.Entries) == 0 {
		return &ers.NotFoundError{Object: filter}
	}

	entry := sr.Entries[0]

	objValue := reflect.ValueOf(obj).Elem()

	// 遍历entry.Attributes（LDAP属性列表）
	for _, attr := range entry.Attributes {
		// 初始化一个空字符串用于存储找到的字段名
		fieldName := ""
		// 遍历结构体的所有字段
		for i := 0; i < objType.NumField(); i++ {
			fieldObj := objType.Field(i)
			tag := fieldObj.Tag.Get("ldap")
			// 如果LDAP标签值与当前属性名相同（不区分大小写），则记录该字段名
			if fieldObj.Type == reflect.TypeOf(BaseObject{}) {
				objectValue := objValue.Field(i)
				for j := 0; j < objectValue.NumField(); j++ {
					structField := objectValue.Type().Field(j)
					objectTag := structField.Tag.Get("ldap")
					fieldName = baseObjType.Field(j).Name
					// 检查Object内的字段的LDAP标签
					if strings.EqualFold(objectTag, attr.Name) {
						objectField := objectValue.Field(j)
						err = setLdapAttr(fieldName, objectField, attr)
						if err != nil {
							return err
						}
					}
				}
			} else if strings.EqualFold(tag, attr.Name) {
				fieldName = objType.Field(i).Name
				// 获取该字段的反射Value
				field := objValue.FieldByName(fieldName)
				err = setLdapAttr(fieldName, field, attr)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
