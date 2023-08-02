package ldap

import (
	"fmt"
	"github.com/go-ldap/ldap"
	"ldap-http-service/lib/ers"
	"ldap-http-service/lib/utils"
	"reflect"
	"strconv"
	"strings"
)

type Group struct {
	BaseObject
	CN                    string   `ldap:"cn" json:"cn"`
	Member                []string `ldap:"member" json:"member"`
	Mail                  string   `ldap:"mail" json:"mail"`
	MailNickName          string   `ldap:"mailNickname" json:"mailNickname"`
	MsExchCoManagedByLink []string `ldap:"msExchCoManagedByLink" json:"msExchCoManagedByLink"`
	ProxyAddresses        []string `ldap:"proxyAddresses" json:"proxyAddresses"`
	ManagedBy             string   `ldap:"managedBy" json:"managedBy"`
	GroupType             string   `ldap:"groupType" json:"groupType"`
}

func (g *Group) ReturnBaseObj() *BaseObject {
	return &g.BaseObject
}

func (g *Group) GetSpecType() reflect.Type {
	return reflect.TypeOf(*g)
}

func (l *ldapConnPool) getGroup(groupId, groupIdType, searchBase string) (group Group, err error) {
	if groupIdType == "objectGUID" {
		groupId, err = unFormatGUID(groupId)
		if err != nil {
			return
		}
	}
	// 设置ldap搜索过滤条件
	filter := fmt.Sprintf("(&(objectClass=group)(objectCategory=group)(%s=%s))", groupIdType, ldap.EscapeFilter(groupId))

	err = l.searchLdapObject(&group, filter, searchBase)

	// 群组成员需要递归获取，单独搜索群组成员
	conn, err := l.getConn()
	if err != nil {
		return
	}
	defer conn.Close()

	group.Member, err = l.getGroupMembers(conn, groupId, groupIdType, 0, []string{})
	return
}

// 递归获取指定群组的所有成员
func (l *ldapConnPool) getGroupMembers(conn *pooledLdapConn, groupId, groupIdType string, pageTop int, members []string) ([]string, error) {
	pageAttr := fmt.Sprintf("member;range=%d-%d", pageTop, pageTop+1500)

	filter := fmt.Sprintf("(&(objectClass=group)(objectCategory=group)(%s=%s))", groupIdType, ldap.EscapeFilter(groupId))

	searchRequest := ldap.NewSearchRequest(
		l.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{pageAttr}, // 使用从结构体标签生成的搜索属性列表
		nil,
	)
	sr, err := conn.Search(searchRequest)
	if err != nil {
		return members, err
	}

	if len(sr.Entries) != 1 {
		return members, &ers.NotFoundError{Object: filter}
	}

	entry := sr.Entries[0]

	if entry.Attributes == nil {
		return members, nil
	}

	for _, attr := range entry.Attributes {
		if utils.RegexFirst(attr.Name, `member;range=(\d+)-(\d+)`) != "" {
			members = append(members, attr.Values...)
			pageTop = pageTop + 1500
		}
		if utils.RegexFirst(attr.Name, `member;range=(\d+)-\*`) != "" {
			members = append(members, attr.Values...)
			return members, nil
		}
	}
	return l.getGroupMembers(conn, groupId, groupIdType, pageTop, members)
}

// 将用户添加到指定的群组
func (l *ldapConnPool) addGroupMembers(groupDN string, userDNs ...string) error {
	// 如果userDNs为空列表或nil，则直接返回
	if len(userDNs) == 0 || userDNs == nil {
		return nil
	}

	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	modifyRequest := ldap.NewModifyRequest(groupDN, []ldap.Control{})
	// 添加成员
	modifyRequest.Add("member", userDNs)

	err = conn.Modify(modifyRequest)
	if err != nil {
		return &ers.OptErr{Option: fmt.Sprintf("add member to group '%s'", groupDN), Message: err.Error()}
	}

	return nil
}

// 从指定群组中移除用户
func (l *ldapConnPool) removeGroupMembers(groupDN string, userDNs ...string) error {
	// 如果userDNs为空列表或nil，则直接返回，传入空列表/nil会导致删除所有成员
	if len(userDNs) == 0 || userDNs == nil {
		return nil
	}

	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	modifyRequest := ldap.NewModifyRequest(groupDN, []ldap.Control{})
	// 移除成员
	modifyRequest.Delete("member", userDNs)

	err = conn.Modify(modifyRequest)
	if err != nil {
		return &ers.OptErr{Option: fmt.Sprintf("remove member from group '%s'", groupDN), Message: err.Error()}
	}

	return nil
}

// 创建ldap群组
func (l *ldapConnPool) createGroup(groupDN, displayName, description string, groupType int) error {
	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	groupName := strings.Replace(strings.ToLower(strings.Split(groupDN, ",")[0]), "cn=", "", 1)

	addRequest := ldap.NewAddRequest(groupDN, []ldap.Control{})
	addRequest.Attribute("objectClass", []string{"top", "group"})
	addRequest.Attribute("name", []string{groupName})
	addRequest.Attribute("displayName", []string{displayName})
	addRequest.Attribute("sAMAccountName", []string{groupName})
	addRequest.Attribute("groupType", []string{strconv.Itoa(groupType)})

	if description == "" {
		description = "同步自ldap微服务"
	}
	addRequest.Attribute("description", []string{description})

	err = conn.Add(addRequest)
	if err != nil {
		return fmt.Errorf("failed to create group: %v", err)
	}

	return nil
}
