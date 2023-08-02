package config

type ldapConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	BaseDN   string
	Domain   string
	Zones    []string
	PoolSize int
}

type ginConfig struct {
	Listen string
	Port   int
}
