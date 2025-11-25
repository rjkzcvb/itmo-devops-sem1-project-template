package database

import (
    "database/sql"
    "fmt"
    "log"

    _ "github.com/lib/pq" // Добавлен подчеркивание для импорта драйвера
)

func Connect() (*sql.DB, error) {
    connStr := "host=localhost port=5432 user=validator password=validator dbname=project-sem-1 sslmode=disable"

    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %v", err)
    }

    // Проверяем соединение с базой данных
    err = db.Ping()
    if err != nil {
        return nil, fmt.Errorf("failed to ping database: %v", err)
    }

    // Создаем таблицу если она не существует
    err = createTableIfNotExists(db)
    if err != nil {
        return nil, fmt.Errorf("failed to create table: %v", err)
    }

    log.Println("Successfully connected to database and verified table")
    return db, nil
}

func createTableIfNotExists(db *sql.DB) error {
    createTableSQL := `
    CREATE TABLE IF NOT EXISTS prices (
        id SERIAL PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        category VARCHAR(255) NOT NULL,
        price DECIMAL(10,2) NOT NULL,
        create_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    )`

    _, err := db.Exec(createTableSQL)
    if err != nil {
        return fmt.Errorf("failed to create table: %v", err)
    }

    log.Println("Table 'prices' verified/created successfully")
    return nil
}
