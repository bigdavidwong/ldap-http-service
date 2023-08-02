package config

import (
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"log"
)

var (
	LdapConfig ldapConfig
	GinConfig  ginConfig
)

// LoadConfig 项目模块配置加载，用于项目启动时从yaml中加载所有配置信息
func LoadConfig() {
	err := envconfig.Process("ldap", &LdapConfig)
	err = envconfig.Process("gin", &GinConfig)
	fmt.Println(LdapConfig)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
}
