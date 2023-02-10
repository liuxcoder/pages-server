package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/go-acme/lego/v4/certificate"
	"xorm.io/xorm"

	// register sql driver
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var _ CertDB = xDB{}

var ErrNotFound = errors.New("entry not found")

type xDB struct {
	engine *xorm.Engine
}

func NewXormDB(dbType, dbConn string) (CertDB, error) {
	if !supportedDriver(dbType) {
		return nil, fmt.Errorf("not supported db type '%s'", dbType)
	}
	if dbConn == "" {
		return nil, fmt.Errorf("no db connection provided")
	}

	e, err := xorm.NewEngine(dbType, dbConn)
	if err != nil {
		return nil, err
	}

	if err := e.Sync2(new(Cert)); err != nil {
		return nil, fmt.Errorf("cound not sync db model :%w", err)
	}

	return &xDB{
		engine: e,
	}, nil
}

func (x xDB) Close() error {
	return x.engine.Close()
}

func (x xDB) Put(domain string, cert *certificate.Resource) error {
	log.Trace().Str("domain", cert.Domain).Msg("inserting cert to db")
	c, err := toCert(domain, cert)
	if err != nil {
		return err
	}

	_, err = x.engine.Insert(c)
	return err
}

func (x xDB) Get(domain string) (*certificate.Resource, error) {
	// TODO: do we need this or can we just go with domain name for wildcard cert
	domain = strings.TrimPrefix(domain, ".")

	cert := new(Cert)
	log.Trace().Str("domain", domain).Msg("get cert from db")
	if found, err := x.engine.ID(domain).Get(cert); err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("%w: name='%s'", ErrNotFound, domain)
	}
	return cert.Raw(), nil
}

func (x xDB) Delete(domain string) error {
	log.Trace().Str("domain", domain).Msg("delete cert from db")
	_, err := x.engine.ID(domain).Delete(new(Cert))
	return err
}

func (x xDB) Compact() (string, error) {
	// not needed
	return "", nil
}

// Items return al certs from db, if pageSize is 0 it does not use limit
func (x xDB) Items(page, pageSize int) ([]*Cert, error) {
	// paginated return
	if pageSize > 0 {
		certs := make([]*Cert, 0, pageSize)
		if page >= 0 {
			page = 1
		}
		err := x.engine.Limit(pageSize, (page-1)*pageSize).Find(&certs)
		return certs, err
	}

	// return all
	certs := make([]*Cert, 0, 64)
	err := x.engine.Find(&certs)
	return certs, err
}

// Supported database drivers
const (
	DriverSqlite   = "sqlite3"
	DriverMysql    = "mysql"
	DriverPostgres = "postgres"
)

func supportedDriver(driver string) bool {
	switch driver {
	case DriverMysql, DriverPostgres, DriverSqlite:
		return true
	default:
		return false
	}
}
