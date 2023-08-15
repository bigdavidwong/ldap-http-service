package ldap

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
		logger.LdapLogger.Info("正在初始化ldap连接池...")
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
		}
	})
	return ldapPool
}

func CreateEnabledUser(tractx context.Context, sAMAccountName, displayName, OU, password, primaryDomain string) error {
	initLdapPool()
	// 开始创建用户
	userFields := logrus.Fields{
		"sAMAccountName": sAMAccountName,
		"displayName":    displayName,
		"OU":             OU,
	}

	// 如果要创建的用户所属域不包含在支持的域内，则直接返回错误
	logger.LdapLogger.WithContext(tractx).WithFields(userFields).Info("开始启动启用用户创建，校验所属域是否支持...")
	if !utils.InSliceIC(ldapPool.Zones, primaryDomain) {
		return &ers.UnSupportedErr{Object: primaryDomain, ObjectType: "domain"}
	}

	// 检测用户是否存在
	logger.LdapLogger.WithContext(tractx).Infof("正在校验用户名 `%s` 的可用性...", sAMAccountName)
	ok, _, err := ldapPool.checkAvailability(sAMAccountName)
	if err != nil {
		return errors.Wrapf(err, "校验用户名 `%s` 的可用性失败", sAMAccountName)
	}
	if !ok {
		return &ers.ObjExistError{Object: fmt.Sprintf("sAMAccountName='%s'", sAMAccountName)}
	}

	// 拼凑用户DN
	userDN := fmt.Sprintf("CN=%s,%s", sAMAccountName, OU)

	// 创建用户
	logger.LdapLogger.WithContext(tractx).Infof("校验通过，开始创建用户对象 `%s` ...", userDN)
	err = ldapPool.createUser(userDN, displayName, primaryDomain)
	if err != nil {
		return err
	}

	// 为用户设置密码
	logger.LdapLogger.WithContext(tractx).Infof("正在为用户 `%s` 设置密码...", userDN)

	err = ldapPool.setPassword(userDN, password)
	if err != nil {
		return errors.Wrapf(err, "为用户 `%s` 设置密码失败", sAMAccountName)
	}

	// 启用用户
	logger.LdapLogger.WithContext(tractx).Infof("正在启用用户 `%s` ...", userDN)

	err = ldapPool.modifyObj(userDN, map[string][]string{"userAccountControl": {"512"}})
	if err != nil {
		return errors.Wrapf(err, "启用用户 `%s` 失败", sAMAccountName)
	}

	return nil
}

func GetUser(userId, userIdType, searchBase string) (User, error) {
	return initLdapPool().getUser(userId, userIdType, searchBase)
}

func MoveObjectToOU(dn, newOU string) error {
	return initLdapPool().moveObjectToOU(dn, newOU)
}

func SetUserPwd(tractx context.Context, userId, userIdType, password, searchBase string) error {
	initLdapPool()
	userFields := logrus.Fields{
		userIdType: userId,
	}

	logger.LdapLogger.WithContext(tractx).WithFields(userFields).Info("开始启动用户密码设置设置，正在获取用户信息...")
	user, err := ldapPool.getUser(userId, userIdType, searchBase)
	if err != nil {
		return errors.Wrapf(err, "查询用户 %s='%s' 失败", userIdType, userId)
	}

	logger.LdapLogger.WithContext(tractx).WithFields(userFields).Infof("查询目标用户完成，正在设置 `%s` 的用户密码...", user.DistinguishedName)
	err = ldapPool.setPassword(user.DistinguishedName, password)
	if err != nil {
		return errors.Wrapf(err, "查询用户 %s='%s' 失败", userIdType, userId)
	}

	logger.LdapLogger.WithContext(tractx).WithFields(userFields).Infof("密码设置完成，为用户 `%s` 执行一次账户解锁...", user.DistinguishedName)
	err = ldapPool.unlockAccount(user.DistinguishedName)
	if err != nil {
		return errors.Wrapf(err, "解锁用户 `%s` 失败", user.DistinguishedName)
	}

	return nil
}

func GetGroup(groupId, groupIdType, searchBase string) (Group, error) {
	return initLdapPool().getGroup(groupId, groupIdType, searchBase)
}

func AddGroupMembers(tractx context.Context, groupId, groupIdType string, userDNs ...string) error {
	initLdapPool()
	groupFields := logrus.Fields{
		groupIdType: groupId,
	}

	logger.LdapLogger.WithContext(tractx).WithFields(groupFields).Info("开始启动群组成员添加，获取目标群组信息...")
	group, err := ldapPool.getGroup(groupId, groupIdType, "")
	if err != nil {
		return errors.Wrapf(err, "获取群组 %s='%s' 的信息失败", groupIdType, groupId)
	}

	// 从待添加列表中去除本来就在群组内的用户
	logger.LdapLogger.WithContext(tractx).Info("获取群组信息完成，预校验待添加成员列表...")
	var i = 0
	for range userDNs {
		if utils.InSlice(userDNs[i], group.Member) {
			logger.LdapLogger.WithContext(tractx).Warningf("目标群组成员已存在: `%s`, 从待添加列表中删除...", userDNs[i])
			userDNs = append(userDNs[0:i], userDNs[i+1:]...)
		} else {
			i++
		}
	}
	logger.LdapLogger.WithContext(tractx).Infof("预校验待添加成员列表完成, 实际待添加人员共计 %d 人, 开始将以下人员添加到群组: %s", len(userDNs), userDNs)
	err = ldapPool.addGroupMembers(group.DistinguishedName, userDNs...)
	if err != nil {
		return errors.Wrap(err, "执行群组成员添加失败")
	}

	return nil
}

func RemoveGroupMembers(tractx context.Context, groupId, groupIdType string, userDNs ...string) error {
	initLdapPool()
	groupFields := logrus.Fields{
		groupIdType: groupId,
	}

	logger.LdapLogger.WithContext(tractx).WithFields(groupFields).Info("开始启动群组成员移除，获取目标群组信息...")
	group, err := ldapPool.getGroup(groupId, groupIdType, "")
	if err != nil {
		return errors.Wrapf(err, "获取群组 %s='%s' 的信息失败", groupIdType, groupId)
	}

	// 从待移除列表中去除本来就在群组内的用户
	logger.LdapLogger.WithContext(tractx).Info("获取群组信息完成，预校验待移除成员列表...")
	var i = 0
	for range userDNs {
		if !utils.InSlice(userDNs[i], group.Member) {
			logger.LdapLogger.WithContext(tractx).Warningf("未在目标群组中检索到成员: `%s`, 从待移除列表中删除...", userDNs[i])
			userDNs = append(userDNs[0:i], userDNs[i+1:]...)
		} else {
			i++
		}
	}
	logger.LdapLogger.WithContext(tractx).Infof("预校验待添加成员列表完成, 实际待移除人员共计 %d 人, 开始将以下人员从群组移除: %s", len(userDNs), userDNs)

	err = ldapPool.removeGroupMembers(group.DistinguishedName, userDNs...)
	if err != nil {
		return errors.Wrap(err, "执行群组成员移除失败")
	}

	return nil
}

func CreateGroup(tractx context.Context, sAMAccountName, OU, displayName, description string, groupType int) error {
	initLdapPool()
	groupFields := logrus.Fields{
		"sAMAccountName": sAMAccountName,
		"OU":             OU,
		"displayName":    displayName,
		"description":    description,
		"groupType":      groupType,
	}

	logger.LdapLogger.WithContext(tractx).WithFields(groupFields).Info("开始启动群组创建，校验群组名可用性...")

	// 检测群组名是否可用
	ok, _, err := ldapPool.checkAvailability(sAMAccountName)
	if err != nil {
		return errors.Wrapf(err, "校验群组名 `%s` 的可用性失败", sAMAccountName)
	}
	if !ok {
		return &ers.ObjExistError{Object: fmt.Sprintf("sAMAccountName='%s'", sAMAccountName)}
	}

	groupDN := fmt.Sprintf("CN=%s,%s", sAMAccountName, OU)

	logger.LdapLogger.WithContext(tractx).Info("校验完成，正在执行群组创建...")
	err = ldapPool.createGroup(groupDN, displayName, description, groupType)
	if err != nil {
		return errors.Wrap(err, "创建群组失败")
	}

	return nil
}

func ModifyObj(dn string, replaceAttr map[string][]string) error {
	return ldapPool.modifyObj(dn, replaceAttr)
}

func CheckAvailability(name string) (bool, *BaseObject, error) {
	return initLdapPool().checkAvailability(name)
}
