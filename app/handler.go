package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"ldap-http-service/constants"
	"ldap-http-service/core/ldap"
	"ldap-http-service/lib/ers"
	"net/http"
)

func handleHealthz(c *gin.Context) {
	_, err := ldap.GetUser(c, "Administrator", "sAMAccountName", "")
	if err != nil {
		_ = c.Error(err)
		return
	}
	JsonWithTraceId(c, http.StatusOK, 0, "ok", map[string]interface{}{})
}

func handleCheckAvailability(c *gin.Context) {
	c.Set("opt", "检查用户名可用性")
	name := c.Query("name")

	ok, obj, err := ldap.CheckAvailability(c, name)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if ok {
		JsonWithTraceId(c, http.StatusOK, 0, "ok", map[string]interface{}{})
	} else {
		JsonWithTraceId(c, http.StatusConflict, constants.CodeObjAlreadyExists, "name has been used", map[string]interface{}{"object": obj})
	}
}

func handleGetUser(c *gin.Context) {
	c.Set("opt", "获取LDAP用户信息")
	userId := c.Param("user_id")
	userIdType := c.Query("user_id_type")
	searchBase := c.Query("search_base")

	user, err := ldap.GetUser(c, userId, userIdType, searchBase)
	if err != nil {
		_ = c.Error(err)
		return
	}
	JsonWithTraceId(c, http.StatusOK, 0, "ok", map[string]interface{}{"user": user})
}

func handleNewEnableUser(c *gin.Context) {
	c.Set("opt", "创建启用LDAP用户")
	var user struct {
		SAMAccountName string `json:"sAMAccountName"`
		DisplayName    string `json:"displayName"`
		OU             string `json:"OU"`
		Password       string `json:"password"`
		PrimaryDomain  string `json:"primaryDomain"`
	}

	if err := c.ShouldBindJSON(&user); err != nil {
		_ = c.Error(&ers.InvalidJsonErr{})
		return
	}

	err := ldap.CreateEnabledUser(c, user.SAMAccountName, user.DisplayName, user.OU, user.Password, user.PrimaryDomain)
	if err != nil {
		_ = c.Error(err)
		return
	}
	userInfo, _ := ldap.GetUser(c, user.SAMAccountName, "sAMAccountName", "")
	JsonWithTraceId(c, http.StatusOK, 0, "ok", map[string]interface{}{"user": userInfo})
}

func handleUserPwd(c *gin.Context) {
	c.Set("opt", "设置LDAP用户密码")
	userId := c.Param("user_id")
	userIdType := c.Query("user_id_type")
	searchBase := c.Query("search_base")

	var pwdChanged struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&pwdChanged); err != nil {
		_ = c.Error(&ers.InvalidJsonErr{})
		return
	}

	err := ldap.SetUserPwd(c, userId, userIdType, pwdChanged.Password, searchBase)
	if err != nil {
		_ = c.Error(err)
		return
	}

	JsonWithTraceId(c, http.StatusOK, 0, "ok", nil)
}

func handleUserUpdate(c *gin.Context) {
	c.Set("opt", "更新LDAP用户信息")
	userId := c.Param("user_id")
	userIdType := c.Query("user_id_type")
	searchBase := c.Query("search_base")

	var userUpdated struct {
		SAMAccountName     string   `json:"sAMAccountName"`
		DisplayName        string   `json:"displayName"`
		Description        string   `json:"description"`
		UserAccountControl int      `json:"userAccountControl"`
		ProxyAddresses     []string `json:"proxyAddresses"`
		Mail               string   `json:"mail"`
		OU                 string   `json:"OU"`
	}
	if err := c.ShouldBindJSON(&userUpdated); err != nil {
		_ = c.Error(&ers.InvalidJsonErr{})
		return
	}

	user, err := ldap.GetUser(c, userId, userIdType, searchBase)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 建立一个map来存储要修改的属性
	var replaceAttr = map[string][]string{}
	if userUpdated.SAMAccountName != "" {
		replaceAttr["sAMAccountName"] = []string{userUpdated.SAMAccountName}
	}

	if userUpdated.DisplayName != "" {
		replaceAttr["displayName"] = []string{userUpdated.DisplayName}
	}

	if userUpdated.Description != "" {
		replaceAttr["description"] = []string{userUpdated.Description}
	}

	if userUpdated.UserAccountControl != 0 {
		replaceAttr["userAccountControl"] = []string{fmt.Sprintf("%d", userUpdated.UserAccountControl)}
	}

	if len(userUpdated.ProxyAddresses) > 0 {
		replaceAttr["proxyAddresses"] = userUpdated.ProxyAddresses
	}

	if userUpdated.Mail != "" {
		replaceAttr["mail"] = []string{userUpdated.Mail}
	}

	err = ldap.ModifyObj(user.DistinguishedName, replaceAttr)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 如果要修改OU，则最后单独修改
	if userUpdated.OU != "" {
		err = ldap.MoveObjectToOU(c, user.DistinguishedName, userUpdated.OU)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}

	user, _ = ldap.GetUser(c, userId, userIdType, searchBase)

	JsonWithTraceId(c, http.StatusOK, 0, "ok", map[string]interface{}{"user": user})
}

func handleGetGroup(c *gin.Context) {
	c.Set("opt", "获取LDAP群组信息")

	groupId := c.Param("group_id")
	groupIdType := c.Query("group_id_type")
	searchBase := c.Query("search_base")

	group, err := ldap.GetGroup(c, groupId, groupIdType, searchBase)
	if err != nil {
		_ = c.Error(err)
		return
	}

	JsonWithTraceId(c, http.StatusOK, 0, "ok", map[string]interface{}{"group": group})
}

func handleNewGroup(c *gin.Context) {
	c.Set("opt", "创建LDAP群组")
	var group struct {
		DisplayName    string `json:"displayName"`
		OU             string `json:"OU"`
		SAMAccountName string `json:"sAMAccountName"`
		Description    string `json:"description"`
		GroupType      int    `json:"groupType"`
	}
	if err := c.ShouldBindJSON(&group); err != nil {
		_ = c.Error(&ers.InvalidJsonErr{})
		return
	}

	err := ldap.CreateGroup(c, group.SAMAccountName, group.OU, group.DisplayName, group.Description, group.GroupType)
	if err != nil {
		_ = c.Error(err)
		return
	}

	groupInfo, _ := ldap.GetGroup(c, group.SAMAccountName, "sAMAccountName", "")
	JsonWithTraceId(c, http.StatusOK, 0, "ok", map[string]interface{}{"group": groupInfo})
}

func handleGroupMemberUpdate(c *gin.Context) {
	c.Set("opt", "更新LDAP群组成员")

	groupId := c.Param("group_id")
	groupIdType := c.Query("group_id_type")
	memberIdType := c.Query("member_id_type")
	searchBase := c.Query("search_base")

	var memberChanged struct {
		AddMembers    []string `json:"add_members"`
		RemoveMembers []string `json:"remove_members"`
	}
	if err := c.ShouldBindJSON(&memberChanged); err != nil {
		_ = c.Error(&ers.InvalidJsonErr{})
		return
	}

	// 查找群组
	group, err := ldap.GetGroup(c, groupId, groupIdType, searchBase)
	if err != nil {
		_ = c.Error(err)
		return
	}

	var (
		message       = "ok"
		addMembers    []string
		removeMembers []string
	)

	var user ldap.User

	for _, am := range memberChanged.AddMembers {
		user, err = ldap.GetUser(c, am, memberIdType, searchBase)
		if err != nil {
			message = fmt.Sprintf("%s;add failed: %s", message, err.Error())
			continue
		}
		addMembers = append(addMembers, user.DistinguishedName)
	}

	for _, rm := range memberChanged.RemoveMembers {
		user, err = ldap.GetUser(c, rm, memberIdType, searchBase)
		if err != nil {
			message = fmt.Sprintf("%s;remove failed: %s", message, err.Error())
			continue
		}
		removeMembers = append(removeMembers, user.DistinguishedName)
	}

	if len(addMembers) > 0 {
		err = ldap.AddGroupMembers(c, group.DistinguishedName, "distinguishedName", addMembers...)
		if err != nil {
			message = fmt.Sprintf("%s;add failed%s", message, err.Error())

		}
	}

	if len(removeMembers) > 0 {
		err = ldap.RemoveGroupMembers(c, group.DistinguishedName, "distinguishedName", removeMembers...)
		if err != nil {
			message = fmt.Sprintf("%s;remove failed: %s", message, err.Error())
		}
	}

	group, _ = ldap.GetGroup(c, groupId, groupIdType, searchBase)
	JsonWithTraceId(c, http.StatusOK, 0, message, map[string]interface{}{"group": group})
}

func handleGroupUpdate(c *gin.Context) {
	c.Set("opt", "更新LDAP群组信息")

	groupId := c.Param("group_id")
	groupIdType := c.Query("group_id_type")
	searchBase := c.Query("search_base")

	var groupUpdated struct {
		DisplayName    string   `json:"displayName"`
		Description    string   `json:"description"`
		ProxyAddresses []string `json:"proxyAddresses"`
		Mail           string   `json:"mail"`
	}
	if err := c.ShouldBindJSON(&groupUpdated); err != nil {
		_ = c.Error(&ers.InvalidJsonErr{})
	}

	group, err := ldap.GetGroup(c, groupId, groupIdType, searchBase)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 建立一个map来存储要修改的属性
	var replaceAttr = map[string][]string{}

	if groupUpdated.DisplayName != "" {
		replaceAttr["displayName"] = []string{groupUpdated.DisplayName}
	}

	if groupUpdated.Description != "" {
		replaceAttr["description"] = []string{groupUpdated.Description}
	}

	if len(groupUpdated.ProxyAddresses) > 0 {
		replaceAttr["proxyAddresses"] = groupUpdated.ProxyAddresses
	}

	if groupUpdated.Mail != "" {
		replaceAttr["mail"] = []string{groupUpdated.Mail}
	}

	err = ldap.ModifyObj(group.DistinguishedName, replaceAttr)
	if err != nil {
		_ = c.Error(err)
		return
	}

	group, _ = ldap.GetGroup(c, groupId, groupIdType, searchBase)

	JsonWithTraceId(c, http.StatusOK, 0, "ok", map[string]interface{}{"group": group})
}
