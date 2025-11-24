package models

import "time"

type PriceRecord struct {
	ProductID   string    `json:"product_id"`
	CreatedDate time.Time `json:"created_date"`
	ProductName string    `json:"product_name"`
	Category    string    `json:"category"`
	Price       float64   `json:"price"`
}

type UploadStats struct {
	TotalItems      int     `json:"total_items"`
	TotalCategories int     `json:"total_categories"`
	TotalPrice      float64 `json:"total_price"`
}
