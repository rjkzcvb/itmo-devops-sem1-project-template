package server

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project/internal/models"
)

func (s *Server) UploadPrices(w http.ResponseWriter, r *http.Request) {
	// Check content type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/zip" {
		http.Error(w, "Expected zip file, got: "+contentType, http.StatusBadRequest)
		return
	}

	// Read the zip file from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	// Process the zip file
	stats, err := s.processZipFile(body)
	if err != nil {
		log.Printf("Error processing zip file: %v", err)
		http.Error(w, "Error processing zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return statistics
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) processZipFile(zipData []byte) (*models.UploadStats, error) {
	// Create zip reader from memory
	zipReader, err := zip.NewReader(strings.NewReader(string(zipData)), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %v", err)
	}

	// Find CSV file
	var csvFile *zip.File
	for _, file := range zipReader.File {
		if strings.HasSuffix(file.Name, ".csv") {
			csvFile = file
			break
		}
	}

	if csvFile == nil {
		fileList := make([]string, len(zipReader.File))
		for i, file := range zipReader.File {
			fileList[i] = file.Name
		}
		return nil, fmt.Errorf("CSV file not found in zip. Files in archive: %v", fileList)
	}

	log.Printf("Found CSV file: %s", csvFile.Name)

	// Open and read CSV file
	fileReader, err := csvFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open csv: %v", err)
	}
	defer fileReader.Close()

	// Parse CSV and insert into database
	return s.parseAndInsertCSV(fileReader)
}

func (s *Server) parseAndInsertCSV(reader io.Reader) (*models.UploadStats, error) {
	csvReader := csv.NewReader(reader)
	
	// 1. Сначала читаем и валидируем все записи из CSV
	var validRecords []*models.PriceRecord
	lineNumber := 0
	
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record at line %d: %v", lineNumber+1, err)
		}

		lineNumber++

		// Skip empty lines
		if len(record) == 0 {
			continue
		}

		// Skip header row
		if lineNumber == 1 {
			if record[0] == "id" || record[0] == "name" || strings.Contains(strings.ToLower(record[0]), "id") {
				continue
			}
		}

		// Validate record length
		if len(record) != 4 {
			return nil, fmt.Errorf("invalid CSV format at line %d: expected 4 fields, got %d", lineNumber, len(record))
		}

		// Parse and validate record
		priceRecord, err := parseCSVRecord(record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse record at line %d: %v", lineNumber, err)
		}

		validRecords = append(validRecords, priceRecord)
	}

	if len(validRecords) == 0 {
		return nil, fmt.Errorf("no valid records found in CSV")
	}

	// 2. Только после валидации открываем транзакцию
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Prepare insert statement
	stmt, err := tx.Prepare(`
		INSERT INTO prices (name, category, price, create_date) 
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Insert all valid records
	for _, record := range validRecords {
		_, err = stmt.Exec(
			record.ProductName,
			record.Category,
			record.Price,
			record.CreatedDate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert record: %v", err)
		}
	}

	// 3. Считаем статистику по всей БД агрегирующими запросами
	stats, err := s.calculateStats(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate statistics: %v", err)
	}
	
	stats.TotalItems = len(validRecords) // Только количество загруженных записей

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return stats, nil
}

func (s *Server) calculateStats(tx *sql.Tx) (*models.UploadStats, error) {
	stats := &models.UploadStats{}

	// Считаем общую сумму цен по всей БД
	err := tx.QueryRow(`
		SELECT COALESCE(SUM(price), 0) 
		FROM prices
	`).Scan(&stats.TotalPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate total price: %v", err)
	}

	// Считаем количество уникальных категорий по всей БД
	err = tx.QueryRow(`
		SELECT COUNT(DISTINCT category) 
		FROM prices
	`).Scan(&stats.TotalCategories)
	if err != nil {
		return nil, fmt.Errorf("failed to count categories: %v", err)
	}

	return stats, nil
}

func parseCSVRecord(record []string) (*models.PriceRecord, error) {
	// [0] = name (product_name)
	// [1] = category
	// [2] = price
	// [3] = create_date

	// Parse date
	createdDate, err := time.Parse("2006-01-02", record[3])
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %s", record[3])
	}

	// Parse price
	price, err := strconv.ParseFloat(record[2], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid price format: %s", record[2])
	}

	return &models.PriceRecord{
		ProductName: record[0],
		Category:    record[1],
		Price:       price,
		CreatedDate: createdDate,
	}, nil
}

func (s *Server) DownloadPrices(w http.ResponseWriter, r *http.Request) {
	// 1. Сначала вычитываем все данные из курсора
	rows, err := s.db.Query(`
		SELECT name, category, price, create_date 
		FROM prices 
		ORDER BY id
	`)
	if err != nil {
		http.Error(w, "Failed to query database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// 2. Сохраняем все данные в память
	var records []struct {
		name       string
		category   string
		price      float64
		createDate time.Time
	}

	for rows.Next() {
		var rec struct {
			name       string
			category   string
			price      float64
			createDate time.Time
		}

		err := rows.Scan(&rec.name, &rec.category, &rec.price, &rec.createDate)
		if err != nil {
			http.Error(w, "Failed to scan row: "+err.Error(), http.StatusInternalServerError)
			return
		}
		records = append(records, rec)
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Error iterating rows: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Только после чтения всех данных создаем CSV
	var csvData strings.Builder
	csvData.WriteString("name,category,price,create_date\n")

	for _, record := range records {
		csvData.WriteString(fmt.Sprintf("%s,%s,%.2f,%s\n",
			record.name,
			record.category,
			record.price,
			record.createDate.Format("2006-01-02"),
		))
	}

	// Create ZIP archive in memory
	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)

	// Add CSV file to ZIP
	csvWriter, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Failed to create CSV in zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = csvWriter.Write([]byte(csvData.String()))
	if err != nil {
		http.Error(w, "Failed to write CSV to zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Close ZIP writer
	err = zipWriter.Close()
	if err != nil {
		http.Error(w, "Failed to close zip: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"prices.zip\"")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", zipBuffer.Len()))

	// Send ZIP file
	_, err = w.Write(zipBuffer.Bytes())
	if err != nil {
		log.Printf("Failed to send zip file: %v", err)
	}
}
