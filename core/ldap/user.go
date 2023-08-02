package ldap

import (
	"fmt"
	"github.com/go-ldap/ldap"
	"golang.org/x/text/encoding/unicode"
	"ldap-http-service/lib/ers"
	"ldap-http-service/lib/utils"
	"reflect"
	"strings"
)

type User struct {
	BaseObject
	Company                    string   `ldap:"company" json:"company"`
	Department                 string   `ldap:"department" json:"department"`
	PhysicalDeliveryOfficeName string   `ldap:"physicalDeliveryOfficeName" json:"physicalDeliveryOfficeName"`
	MemberOf                   []string `ldap:"memberOf" json:"memberOf"`
	PwdLastSet                 FileTime `ldap:"pwdLastSet" json:"pwdLastSet"`
	LockoutTime                FileTime `ldap:"lockoutTime" json:"lockoutTime"`
	LastLogon                  FileTime `ldap:"lastLogon" json:"lastLogon"`
	PwdExpiryTime              FileTime `ldap:"msDS-UserPasswordExpiryTimeComputed" json:"msDS-UserPasswordExpiryTimeComputed"`
	ProxyAddresses             []string `ldap:"proxyAddresses" json:"proxyAddresses"`
	Mail                       string   `ldap:"mail" json:"mail"`
	MailNickname               string   `ldap:"mailNickname" json:"mailNickname"`
	UserPrincipalName          string   `ldap:"userPrincipalName" json:"userPrincipalName"`
	UserAccountControl         string   `ldap:"userAccountControl" json:"userAccountControl"`
	LegacyExchangeDN           string   `ldap:"legacyExchangeDN" json:"legacyExchangeDN"`
	HomeMDB                    string   `ldap:"homeMDB" json:"homeMDB"`
	MDBUseDefaults             bool     `ldap:"mDBUseDefaults" json:"mDBUseDefaults"`
	MDBStorageQuota            int64    `ldap:"mDBStorageQuota" json:"mDBStorageQuota"`
	MDBOverQuotaLimit          int64    `ldap:"mDBOverQuotaLimit" json:"mDBOverQuotaLimit"`
	MDBOverHardQuotaLimit      int64    `ldap:"mDBOverHardQuotaLimit" json:"mDBOverHardQuotaLimit"`
}

func (u *User) ReturnBaseObj() *BaseObject {
	return &u.BaseObject
}

func (u *User) GetSpecType() reflect.Type {
	return reflect.TypeOf(*u)
}

// 创建用户
func (l *ldapConnPool) createUser(userDN, displayName, primaryDomain string) error {
	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	username := strings.Replace(strings.ToLower(strings.Split(userDN, ",")[0]), "cn=", "", 1)
	if primaryDomain == "" {
		primaryDomain = l.Domain
	}

	addRequest := ldap.NewAddRequest(userDN, []ldap.Control{})
	addRequest.Attribute("objectClass", []string{"top", "user", "organizationalPerson", "person"})
	addRequest.Attribute("name", []string{username})
	addRequest.Attribute("displayName", []string{displayName})
	addRequest.Attribute("sAMAccountName", []string{username})
	addRequest.Attribute("instanceType", []string{fmt.Sprintf("%d", 0x00000004)})
	addRequest.Attribute("userAccountControl", []string{fmt.Sprintf("%d", 0x0202)})
	addRequest.Attribute("userPrincipalName", []string{fmt.Sprintf("%s@%s", username, primaryDomain)})
	addRequest.Attribute("accountExpires", []string{fmt.Sprintf("%d", 0x00000000)})
	err = conn.Add(addRequest)
	if err != nil {
		return &ers.OptErr{Option: "create user", Message: err.Error()}
	}
	return nil
}

// 设置用户密码
func (l *ldapConnPool) setPassword(userDN, password string) error {
	username := strings.Replace(strings.ToLower(strings.Split(userDN, ",")[0]), "cn=", "", 1)
	if !utils.IsStrongPassword(username, password) {
		return &ers.OptErr{Option: fmt.Sprintf("set password for '%s'", username), Message: "password is not strong enough"}
	}

	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	pwdEncoded, err := utf16.NewEncoder().String(fmt.Sprintf("%q", password))
	if err != nil {
		return err
	}

	modReq := ldap.NewModifyRequest(userDN, []ldap.Control{})
	modReq.Replace("unicodePwd", []string{pwdEncoded})

	if err = conn.Modify(modReq); err != nil {
		return &ers.OptErr{Option: fmt.Sprintf("set password for '%s'", userDN), Message: err.Error()}
	}
	return nil
}

// 启用用户
func (l *ldapConnPool) enableUser(userDN string) error {
	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	modReq := ldap.NewModifyRequest(userDN, []ldap.Control{})
	modReq.Replace("userAccountControl", []string{"512"})

	if err = conn.Modify(modReq); err != nil {
		return &ers.OptErr{Option: fmt.Sprintf("enable user '%s'", userDN), Message: err.Error()}
	}
	return nil
}

// 解锁用户
func (l *ldapConnPool) unlockAccount(userDN string) error {
	conn, err := l.getConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	modReq := ldap.NewModifyRequest(userDN, []ldap.Control{})
	modReq.Replace("lockoutTime", []string{"0"})

	if err = conn.Modify(modReq); err != nil {
		return &ers.OptErr{Option: fmt.Sprintf("unlock user '%s'", userDN), Message: err.Error()}
	}
	return nil
}

// 根据结构体获取用户信息
func (l *ldapConnPool) getUser(userId, userIdType, searchBase string) (user User, err error) {
	if userIdType == "objectGUID" {
		userId, err = unFormatGUID(userId)
		if err != nil {
			return
		}
	}

	// 设置ldap过滤查询条件
	filter := fmt.Sprintf("(&(objectClass=user)(objectCategory=person)(%s=%s))", userIdType, ldap.EscapeFilter(userId))

	err = l.searchLdapObject(&user, filter, searchBase)
	return
}
