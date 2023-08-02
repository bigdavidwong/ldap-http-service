package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/go-ldap/ldap"
	"ldap-http-service/lib/ers"
	"ldap-http-service/lib/logger"
	"sync"
	"time"
)

type ldapConnPool struct {
	Host     string
	Port     int
	BaseDN   string
	Domain   string
	Zones    []string
	username string
	password string
	pool     chan *pooledLdapConn
	counter  int
	mutex    sync.Mutex
	LOGGER   *logger.Logger
}

type pooledLdapConn struct {
	*ldap.Conn
	pool *ldapConnPool
	SN   int
}

func (c *pooledLdapConn) Close() {
	c.pool.putConn(c)
}

func (c *pooledLdapConn) IsAlive() bool {
	_, err := c.Search(&ldap.SearchRequest{
		BaseDN:     c.pool.BaseDN,
		Scope:      ldap.ScopeBaseObject,
		Filter:     "(&(objectClass=user)(objectCategory=person)(name=Administrator))",
		Attributes: []string{"name"},
	})
	if err != nil {
		return false
	} else {
		return true
	}
}

func (l *ldapConnPool) newConn(SN int) (*pooledLdapConn, error) {
	address := fmt.Sprintf("%s:%d", l.Host, l.Port)
	ldapConn, err := ldap.DialTLS("tcp", address, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil, err
	}

	err = ldapConn.Bind(l.username, l.password)
	if err != nil {
		return nil, err
	}
	return &pooledLdapConn{ldapConn, l, SN}, nil
}

func (l *ldapConnPool) getConn() (conn *pooledLdapConn, err error) {
	timeOut := 5 * time.Second
	timeout, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()

	select {
	case conn = <-l.pool:
		if conn.IsAlive() {
			return conn, nil
		} else {
			conn.Conn.Close()
			return l.newConn(conn.SN)
		}

	default:
		l.mutex.Lock()
		defer l.mutex.Unlock()
		if l.counter < cap(l.pool) {
			conn, err = l.newConn(l.counter)
			if err != nil {
				return
			} else {
				l.counter++
			}
		} else {
			select {
			case conn = <-l.pool:
				if conn.IsAlive() {
					return conn, nil
				} else {
					conn.Conn.Close()
					return l.newConn(conn.SN)
				}
			case <-timeout.Done():
				err = &ers.TimeoutErr{Option: "get ldap conn", Time: timeOut}
			}
		}
	}
	return
}

func (l *ldapConnPool) putConn(conn *pooledLdapConn) {
	select {
	case l.pool <- conn:
	default:
		conn.Conn.Close()
	}
}
