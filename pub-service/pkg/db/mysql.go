package db

import (
	"context"
	"fmt"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MySQLDB struct {
	db *gorm.DB
}

func NewMySQLDB(ctx context.Context) (*MySQLDB, error) {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "mysql"
	}

	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		return nil, fmt.Errorf("DB_USER is required")
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}

	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		return nil, fmt.Errorf("DB_NAME is required")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, dbname)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &MySQLDB{
		db: db,
	}, nil
}

func (p *MySQLDB) GetDB() *gorm.DB {
	return p.db
}

func (p *MySQLDB) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
