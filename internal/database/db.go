package database

import (
	// "database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/noahjalex/epoch/internal/models"
	"github.com/sirupsen/logrus"
)

type DB struct {
	*sqlx.DB
}

func SetupDB(log *logrus.Logger) (*DB, *models.Repo) {
	// Database configuration
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "epoch")
	dbPassword := getEnv("DB_PASSWORD", "devpass")
	dbName := getEnv("DB_NAME", "epoch")

	log.WithFields(map[string]interface{}{
		"db_host": dbHost,
		"db_port": dbPort,
		"db_user": dbUser,
		"db_name": dbName,
	}).Info("Database configuration loaded")

	// Connect to database
	db, err := new(dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}

	log.Info("Database connection established")

	// // Run migrations
	// if err := db.RunMigrations("migrations"); err != nil {
	// 	log.WithError(err).Fatal("Failed to run migrations")
	// }
	//
	// log.Info("Database migrations completed")

	// Initialize repository and handlers
	repo := models.NewRepository(db.DB)
	return db, repo
}

func new(host, port, user, password, dbname string) (*DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sqlx.Open("postgres", psqlInfo)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (db *DB) RunMigrations(migrationsPath string) error {
	files, err := filepath.Glob(filepath.Join(migrationsPath, "*.sql"))
	if err != nil {
		return err
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("error running migration %s: %v", file, err)
		}

	}
	return nil
}

func getEnv(name string, def string) string {
	val := os.Getenv(name)
	if val == "" {
		return def
	}
	return val
}
