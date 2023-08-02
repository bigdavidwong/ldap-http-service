package ldap

import (
	"context"
	"fmt"
	"ldap-http-service/config"
	"ldap-http-service/lib/ers"
	"ldap-http-service/lib/logger"
	"ldap-http-service/lib/utils"
	"sync"
)

var (
	ldapPool *ldapConnPool
	ldapOnce sync.Once
)

func initLdapPool() *ldapConnPool {
	ldapOnce.Do(func() {
		LOGGER, err := logger.NewLogger("ldap")
		if err != nil {
			panic(fmt.Sprintf("Logger初始化失败：%v", err))
		}
		ldapPool = &ldapConnPool{
			Host:     config.LdapConfig.Host,
			Port:     config.LdapConfig.Port,
			username: config.LdapConfig.Username,
			password: config.LdapConfig.Password,
			BaseDN:   config.LdapConfig.BaseDN,
			Domain:   config.LdapConfig.Domain,
			Zones:    config.LdapConfig.Zones,
			pool:     make(chan *pooledLdapConn, config.LdapConfig.PoolSize),
			counter:  0,
			LOGGER:   LOGGER,
		}
	})
	return ldapPool
}

func CreateEnabledUser(tractx context.Context, sAMAccountName, displayName, OU, password, primaryDomain string) error {
	initLdapPool()
	// 如果要创建的用户所属域不包含在支持的域内，则直接返回错误
	if !utils.InSliceIC(ldapPool.Zones, primaryDomain) {
		return &ers.UnSupportedErr{Object: primaryDomain, ObjectType: "domain"}
	}

	// 检测用户是否存在
	_, err := ldapPool.getUser(sAMAccountName, "sAMAccountName", "")
	if _, ok := err.(*ers.NotFoundError); !ok {
		return &ers.ObjExistError{Object: fmt.Sprintf("sAMAccountName='%s'", sAMAccountName)}
	}

	// 拼凑用户DN
	userDN := fmt.Sprintf("CN=%s,%s", sAMAccountName, OU)

	// 创建用户
	ldapPool.LOGGER.Info(tractx, fmt.Sprintf("开始创建用户 <%s> ...", userDN))
	err = ldapPool.createUser(userDN, displayName, primaryDomain)
	if err != nil {
		return err
	}

	// 为用户设置密码
	ldapPool.LOGGER.Info(tractx, fmt.Sprintf("为用户 <%s> 设置密码...", userDN))
	err = ldapPool.setPassword(userDN, password)
	if err != nil {
		return err
	}

	// 启用用户
	ldapPool.LOGGER.Info(tractx, fmt.Sprintf("正在启用用户 <%s> ...", userDN))
	return ldapPool.modifyObj(userDN, map[string][]string{"userAccountControl": {"512"}})
}

func GetUser(userId, userIdType, searchBase string) (User, error) {
	return initLdapPool().getUser(userId, userIdType, searchBase)
}

func MoveObjectToOU(dn, newOU string) error {
	return initLdapPool().moveObjectToOU(dn, newOU)
}

func SetUserPwd(tractx context.Context, userId, userIdType, password, searchBase string) error {
	initLdapPool()
	ldapPool.LOGGER.Info(tractx, fmt.Sprintf("正在获取 %s='%s' 的用户信息...", userIdType, userId))
	user, err := ldapPool.getUser(userId, userIdType, searchBase)
	if err != nil {
		return err
	}

	ldapPool.LOGGER.Info(tractx, fmt.Sprintf("正在设置 <%s> 的ldap密码...", user.DistinguishedName))
	err = ldapPool.setPassword(user.DistinguishedName, password)
	if err != nil {
		return err
	}

	ldapPool.LOGGER.Info(tractx, fmt.Sprintf("密码设置完成，为账户 <%s> 解锁...", user.DistinguishedName))
	return ldapPool.unlockAccount(user.DistinguishedName)
}

func GetGroup(groupId, groupIdType, searchBase string) (Group, error) {
	return initLdapPool().getGroup(groupId, groupIdType, searchBase)
}

func AddGroupMembers(tractx context.Context, groupId, groupIdType string, userDNs ...string) error {
	group, err := initLdapPool().getGroup(groupId, groupIdType, "")
	if err != nil {
		return err
	}

	// 去除本来就在群组内的用户
	ldapPool.LOGGER.Info(tractx, "预校验待添加成员列表...")
	var i = 0
	for range userDNs {
		if utils.InSlice(userDNs[i], group.Member) {
			ldapPool.LOGGER.Warning(tractx, fmt.Sprintf("目标群组成员已存在: <%s>, 从待添加列表中删除...", userDNs[i]))
			userDNs = append(userDNs[0:i], userDNs[i+1:]...)
		} else {
			i++
		}
	}
	ldapPool.LOGGER.Debug(tractx, fmt.Sprintf("预校验待添加成员列表完成, 开始移除以下成员: %v", userDNs))
	return ldapPool.addGroupMembers(group.DistinguishedName, userDNs...)
}

func RemoveGroupMembers(tractx context.Context, groupId, groupIdType string, userDNs ...string) error {
	group, err := initLdapPool().getGroup(groupId, groupIdType, "")
	if err != nil {
		return err
	}

	// 去除本来就不在群组内的用户
	ldapPool.LOGGER.Info(tractx, "预校验待移除成员列表...")
	var i = 0
	for range userDNs {
		if !utils.InSlice(userDNs[i], group.Member) {
			ldapPool.LOGGER.Warning(tractx, fmt.Sprintf("未在目标群组中检索到成员: <%s>, 从待移除列表中删除...", userDNs[i]))
			userDNs = append(userDNs[0:i], userDNs[i+1:]...)
		} else {
			i++
		}
	}
	ldapPool.LOGGER.Debug(tractx, fmt.Sprintf("预校验待移除成员列表完成, 开始移除以下成员: %v", userDNs))
	return ldapPool.removeGroupMembers(group.DistinguishedName, userDNs...)
}

func CreateGroup(tractx context.Context, sAMAccountName, OU, displayName, description string, groupType int) error {
	initLdapPool()

	// 检测群组是否存在
	_, err := ldapPool.getGroup(sAMAccountName, "sAMAccountName", "")
	if _, ok := err.(*ers.NotFoundError); !ok {
		return &ers.ObjExistError{Object: fmt.Sprintf("sAMAccountName='%s'", sAMAccountName)}
	}

	groupDN := fmt.Sprintf("CN=%s,%s", sAMAccountName, OU)
	ldapPool.LOGGER.Info(tractx, fmt.Sprintf("开始创建群组: <%s>, 显示名称: %s, 描述: %s, 类型: %d", groupDN, displayName, description, groupType))
	return ldapPool.createGroup(groupDN, displayName, description, groupType)
}

func ModifyObj(dn string, replaceAttr map[string][]string) error {
	return ldapPool.modifyObj(dn, replaceAttr)
}

func CheckAvailability(name string) (bool, BaseObject) {
	return initLdapPool().checkAvailability(name)
}
